package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersFeedForYouParams struct {
	Limit        int `query:"limit" default:"25" validate:"min=1,max=100"`
	Offset       int `query:"offset" default:"0" validate:"min=0,max=200"`
	MaxPerArtist int `query:"max_per_artist" default:"3" validate:"min=1,max=10"`
}

// v1UsersFeedForYou returns a personalized "For You" track feed for the
// user identified in the path. Modeled on Twitter's open-sourced 2023
// algorithm (the-algorithm-ml). The shape of the pipeline is
// candidate-retrieval → ranking → filtering+diversity, the same
// three-stage pattern Twitter uses on top of a learned "heavy ranker."
// Audius doesn't yet have a trained ranker, so the heavy ranker is
// approximated by a hand-tuned linear blend below; the candidate
// retrieval / diversity passes carry over directly so a learned model
// can drop in later.
//
// 1. CANDIDATE RETRIEVAL (UNION across four sources, each capped):
//   - in_network  — tracks uploaded in the last 14 days by users I follow.
//     Strongest "this is for me" signal.
//   - trending    — top week-trending from track_trending_scores.
//     Mirrors GET /tracks/trending. Capped at 100.
//   - underground — week-trending tracks whose owner has < 1500
//     follower & following count (mirrors GET /tracks/trending/underground).
//     Capped at 50.
//   - similar     — recent uploads by artists who are saved by other
//     users that also save artists I save. 1-hop collaborative filter on
//     the saves graph; capped at 50 artists × recent uploads.
//
// 2. RANKING — three light signals combined linearly:
//
//	recency_score    = exp(-ln(2) * age_hours / 48)
//	                   // 48h half-life: 48h-old → 0.5, 96h → 0.25
//	engagement_score = ln(1 + 3*saves + 2*reposts + 1*plays) / 12
//	                   // saves > reposts > plays, log-compressed
//	social_boost     = 1.0 + min(ln(1 + my_engagement_with_artist) / 4, 1)
//	                   // up to ~2x for artists I already engage with often
//	source_weight    = {in_network: 1.20, trending: 1.00,
//	                    underground: 0.95, similar: 0.90}
//
//	final_score = (0.55 * recency_score + 0.45 * engagement_score)
//	              * social_boost * source_weight
//
// 3. FILTERS — applied once after the union to keep the candidate set cheap:
//   - is_delete / is_unlisted / is_available / stem_of (track liveness)
//   - users.is_deactivated / is_available (owner liveness — same shape
//     as v1_events_remix_contests.go)
//   - access_authorities: caller's wallet must be on the list, else
//     ungated only (matches the v1_users_feed authed-wallet pattern)
//   - already-saved by the path-param user (don't resurface)
//   - the path-param user's own uploads
//
// 4. DIVERSITY — author-cap of N tracks per owner via row_number()
// (configurable via max_per_artist; default 3), then a Go-side greedy
// pass that, when the next track shares an owner with the previously
// emitted one, prefers the next non-same-owner candidate within a
// 5-position lookahead. This is a "consecutive same-artist penalty"
// without paying for a second ranker.
//
// PAGINATION is offset/limit applied on the diversity-ordered list, so
// pages are stable as long as the underlying scores haven't shifted.
//
// Path:
//   - id (required): the user being personalized for. Resolved by
//     requireUserIdMiddleware; the handler returns the 400 from that
//     middleware on an invalid hash id.
//
// Query params:
//   - limit          (default 25, max 100)
//   - offset         (default 0,  max 200)
//   - max_per_artist (default 3,  max 10) — author cap per page
//   - user_id        (optional): the caller's id. Independent of the
//     path id — used to populate has_current_user_reposted and similar
//     viewer-relative fields on the returned tracks. Same role it plays
//     on every other /v1/users/{id}/... endpoint.
func (app *ApiServer) v1UsersFeedForYou(c *fiber.Ctx) error {
	var params = GetUsersFeedForYouParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	// Path id — the user we personalize for. Validated by middleware.
	userId := app.getUserId(c)
	// Optional caller id from ?user_id=, used only for viewer-relative
	// track fields on the response shape.
	myId := app.getMyId(c)

	authedWallet := app.tryGetAuthedWallet(c)

	// Pull a candidate pool larger than the page size so the diversity
	// cap and the consecutive-same-artist pass have headroom to reorder.
	const candidatePoolSize = 200

	sql := `
	WITH
	follow_set AS (
		SELECT followee_user_id AS user_id
		FROM follows
		WHERE follower_user_id = @userId
		  AND is_current = true
		  AND is_delete = false
	),
	my_saved_tracks AS (
		SELECT save_item_id AS track_id
		FROM saves
		WHERE user_id = @userId
		  AND save_type = 'track'
		  AND is_current = true
		  AND is_delete = false
	),
	-- The set of artists I save anchors the collaborative-filter join
	-- below. For long-tenure power users with thousands of saved artists
	-- the saves self-join explodes and the planner times out, so we cap
	-- to the 200 most recently saved artists. Recency is the right axis:
	-- old saves are weaker signal of current taste anyway, and a 200-artist
	-- anchor still gives the similar-artists CTE enough to work with.
	my_saved_artists AS (
		SELECT t.owner_id AS artist_id, MAX(s.created_at) AS last_saved_at
		FROM saves s
		JOIN tracks t ON t.track_id = s.save_item_id
		WHERE s.user_id = @userId
		  AND s.save_type = 'track'
		  AND s.is_current = true
		  AND s.is_delete = false
		GROUP BY t.owner_id
		ORDER BY last_saved_at DESC
		LIMIT 200
	),
	-- 1-hop collaborative filter on the saves graph: artists saved by
	-- users who *also* save my saved-artists, but who I haven't saved myself.
	-- Bounded so the planner can't get adventurous on power-users.
	similar_artists AS (
		SELECT t2.owner_id AS artist_id, COUNT(DISTINCT s2.user_id) AS overlap
		FROM saves s1
		JOIN tracks t1 ON s1.save_item_id = t1.track_id
		JOIN saves s2 ON s2.user_id = s1.user_id
		             AND s2.save_type = 'track'
		             AND s2.is_current = true
		             AND s2.is_delete = false
		JOIN tracks t2 ON s2.save_item_id = t2.track_id
		WHERE s1.save_type = 'track'
		  AND s1.is_current = true
		  AND s1.is_delete = false
		  AND s1.user_id <> @userId
		  AND t1.owner_id IN (SELECT artist_id FROM my_saved_artists)
		  AND t2.owner_id NOT IN (SELECT artist_id FROM my_saved_artists)
		GROUP BY t2.owner_id
		ORDER BY overlap DESC
		LIMIT 50
	),
	-- Per-artist engagement strength (saves + reposts + plays of any of
	-- their tracks by me). Used for the social_boost multiplier.
	my_artist_affinity AS (
		SELECT t.owner_id AS artist_id,
		       LN(1 + COUNT(*)) AS affinity
		FROM (
			SELECT save_item_id AS track_id FROM saves
			 WHERE user_id = @userId AND save_type = 'track'
			   AND is_current = true AND is_delete = false
			UNION ALL
			SELECT repost_item_id AS track_id FROM reposts
			 WHERE user_id = @userId AND repost_type = 'track'
			   AND is_current = true AND is_delete = false
			UNION ALL
			SELECT play_item_id AS track_id FROM plays
			 WHERE user_id = @userId
		) eng
		JOIN tracks t ON t.track_id = eng.track_id
		GROUP BY t.owner_id
	),
	-- Source 1: in-network (followed-creator) recent uploads.
	-- Bounded so a power-user with thousands of follows doesn't pull a
	-- multi-thousand-row pool we'd just throw away after ranking.
	cand_in_network AS (
		SELECT t.track_id, 'in_network'::text AS source
		FROM tracks t
		JOIN follow_set f ON f.user_id = t.owner_id
		WHERE t.is_current = true
		  AND t.is_delete = false
		  AND t.is_unlisted = false
		  AND t.is_available = true
		  AND t.stem_of IS NULL
		  AND t.created_at >= NOW() - INTERVAL '14 days'
		ORDER BY t.created_at DESC
		LIMIT 200
	),
	-- Source 2: weekly trending.
	cand_trending AS (
		SELECT tts.track_id, 'trending'::text AS source
		FROM track_trending_scores tts
		JOIN tracks t ON t.track_id = tts.track_id
		            AND t.is_current = true
		            AND t.is_delete = false
		            AND t.is_unlisted = false
		            AND t.is_available = true
		WHERE tts.type = 'TRACKS'
		  AND tts.version = 'pnagD'
		  AND tts.time_range = 'week'
		  AND (tts.genre IS NULL OR tts.genre = '')
		ORDER BY tts.score DESC, tts.track_id DESC
		LIMIT 100
	),
	-- Source 3: underground trending (sub-1500 follower & following artists).
	cand_underground AS (
		SELECT tts.track_id, 'underground'::text AS source
		FROM track_trending_scores tts
		JOIN tracks t ON t.track_id = tts.track_id
		            AND t.is_current = true
		            AND t.is_delete = false
		            AND t.is_unlisted = false
		            AND t.is_available = true
		JOIN aggregate_user au ON au.user_id = t.owner_id
		WHERE tts.type = 'TRACKS'
		  AND tts.version = 'pnagD'
		  AND tts.time_range = 'week'
		  AND (tts.genre IS NULL OR tts.genre = '')
		  AND au.follower_count < 1500
		  AND au.following_count < 1500
		ORDER BY tts.score DESC, tts.track_id DESC
		LIMIT 50
	),
	-- Source 4: similar-artist recent uploads.
	cand_similar AS (
		SELECT t.track_id, 'similar'::text AS source
		FROM tracks t
		JOIN similar_artists sa ON sa.artist_id = t.owner_id
		WHERE t.is_current = true
		  AND t.is_delete = false
		  AND t.is_unlisted = false
		  AND t.is_available = true
		  AND t.stem_of IS NULL
		  AND t.created_at >= NOW() - INTERVAL '60 days'
		ORDER BY sa.overlap DESC, t.created_at DESC
		LIMIT 100
	),
	-- One row per track_id. DISTINCT ON keeps the strongest (lowest-prio)
	-- source so an in-network track that's also trending keeps the
	-- in_network weight rather than the lower trending weight.
	candidates AS (
		SELECT DISTINCT ON (track_id) track_id, source
		FROM (
			SELECT track_id, source, 1 AS prio FROM cand_in_network
			UNION ALL
			SELECT track_id, source, 2 AS prio FROM cand_trending
			UNION ALL
			SELECT track_id, source, 3 AS prio FROM cand_underground
			UNION ALL
			SELECT track_id, source, 4 AS prio FROM cand_similar
		) u
		ORDER BY track_id, prio
	),
	filtered AS (
		SELECT
			c.track_id,
			c.source,
			t.owner_id,
			t.created_at,
			COALESCE(at.save_count, 0)   AS save_count,
			COALESCE(at.repost_count, 0) AS repost_count,
			COALESCE(ap.count, 0)        AS play_count,
			COALESCE(maa.affinity, 0)    AS affinity
		FROM candidates c
		JOIN tracks t ON t.track_id = c.track_id
		JOIN users  u ON u.user_id  = t.owner_id
		LEFT JOIN aggregate_track at  ON at.track_id     = c.track_id
		LEFT JOIN aggregate_plays ap  ON ap.play_item_id = c.track_id
		LEFT JOIN my_artist_affinity maa ON maa.artist_id = t.owner_id
		WHERE t.is_current   = true
		  AND t.is_delete    = false
		  AND t.is_unlisted  = false
		  AND t.is_available = true
		  AND t.stem_of IS NULL
		  -- Owner liveness — pattern from v1_events_remix_contests.go
		  AND u.is_current     = true
		  AND u.is_deactivated = false
		  AND u.is_available   = true
		  -- Access-gating: ungated, or caller's wallet is on the list
		  AND (t.access_authorities IS NULL
		       OR (COALESCE(@authed_wallet, '') <> ''
		           AND EXISTS (
		               SELECT 1 FROM unnest(t.access_authorities) aa
		               WHERE lower(aa) = lower(@authed_wallet)
		           )))
		  -- Don't resurface tracks the caller already saved
		  AND NOT EXISTS (
		      SELECT 1 FROM my_saved_tracks ms WHERE ms.track_id = c.track_id
		  )
		  -- Don't recommend the caller's own uploads
		  AND t.owner_id <> @userId
	),
	scored AS (
		SELECT
			f.track_id,
			f.owner_id,
			-- 48h half-life on age in hours
			EXP(-LN(2) * GREATEST(EXTRACT(EPOCH FROM (NOW() - f.created_at)) / 3600.0, 0) / 48.0)
				AS recency_score,
			-- saves > reposts > plays, log-compressed and normalized
			LN(1 + 3 * f.save_count + 2 * f.repost_count + f.play_count) / 12.0
				AS engagement_score,
			-- 1.0 baseline, up to ~2x for high-affinity artists
			1.0 + LEAST(f.affinity / 4.0, 1.0) AS social_boost,
			CASE f.source
				WHEN 'in_network'  THEN 1.20
				WHEN 'trending'    THEN 1.00
				WHEN 'underground' THEN 0.95
				WHEN 'similar'     THEN 0.90
				ELSE 1.00
			END AS source_weight
		FROM filtered f
	),
	final_scored AS (
		SELECT
			track_id,
			owner_id,
			(0.55 * recency_score + 0.45 * engagement_score)
				* social_boost * source_weight AS score
		FROM scored
	),
	-- Hard cap: max 3 tracks per artist before paginating.
	capped AS (
		SELECT track_id, owner_id, score,
		       ROW_NUMBER() OVER (PARTITION BY owner_id ORDER BY score DESC, track_id DESC)
		         AS rn_artist
		FROM final_scored
	)
	SELECT track_id, owner_id
	FROM capped
	WHERE rn_artist <= @maxPerArtist
	ORDER BY score DESC, track_id DESC
	LIMIT @poolSize
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId":        userId,
		"poolSize":      candidatePoolSize,
		"maxPerArtist":  params.MaxPerArtist,
		"authed_wallet": authedWallet,
	})
	if err != nil {
		return err
	}

	type ranked struct {
		TrackID int32
		OwnerID int32
	}
	pool, err := pgx.CollectRows(rows, pgx.RowToStructByPos[ranked])
	if err != nil {
		return err
	}

	// Greedy diversity pass: keep the global rank order, but if the next
	// track shares an owner with the one just emitted, prefer the next
	// non-same-owner candidate within a small lookahead. Soft penalty on
	// consecutive-same-artist runs without computing a second ranker.
	const lookahead = 5
	ordered := make([]ranked, 0, len(pool))
	used := make([]bool, len(pool))
	var lastOwner int32 = -1
	for n := 0; n < len(pool); n++ {
		pickIdx := -1
		for i := 0; i < len(pool) && i < n+lookahead+1; i++ {
			if used[i] {
				continue
			}
			if pickIdx == -1 {
				pickIdx = i
			}
			if pool[i].OwnerID != lastOwner {
				pickIdx = i
				break
			}
		}
		if pickIdx == -1 {
			break
		}
		used[pickIdx] = true
		ordered = append(ordered, pool[pickIdx])
		lastOwner = pool[pickIdx].OwnerID
	}

	// Apply pagination on the diversity-ordered list.
	start := params.Offset
	if start > len(ordered) {
		start = len(ordered)
	}
	end := start + params.Limit
	if end > len(ordered) {
		end = len(ordered)
	}
	page := ordered[start:end]

	trackIds := make([]int32, len(page))
	for i, r := range page {
		trackIds[i] = r.TrackID
	}

	tracks, err := app.queries.Tracks(c.Context(), dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:          trackIds,
			MyID:         myId,
			AuthedWallet: authedWallet,
		},
	})
	if err != nil {
		return err
	}

	// Tracks() returns rows in id order; re-emit in our ranked order.
	byId := make(map[int32]dbv1.Track, len(tracks))
	for _, t := range tracks {
		byId[t.TrackID] = t
	}
	sorted := make([]dbv1.Track, 0, len(trackIds))
	for _, id := range trackIds {
		if t, ok := byId[id]; ok {
			sorted = append(sorted, t)
		}
	}

	return v1TracksResponse(c, sorted)
}
