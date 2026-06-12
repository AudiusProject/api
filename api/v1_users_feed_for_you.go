package api

import (
	"time"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersFeedForYouParams struct {
	Limit        int `query:"limit" default:"25" validate:"min=1,max=100"`
	Offset       int `query:"offset" default:"0" validate:"min=0,max=200"`
	MaxPerArtist int `query:"max_per_artist" default:"3" validate:"min=1,max=10"`
}

// v1FeedForYou returns a personalized "For You" feed for the user
// identified in the path. The response is a heterogenous list of
// tracks and playlists/albums (`{type, timestamp, item}` items),
// mirroring the chronological /v1/users/{id}/feed shape so clients can
// render both in a single ranked list.
//
// Modeled on Twitter's open-sourced 2023 algorithm (the-algorithm-ml).
// The shape of the pipeline is candidate-retrieval → ranking →
// filtering+diversity, the same three-stage pattern Twitter uses on top
// of a learned "heavy ranker." Audius doesn't yet have a trained ranker,
// so the heavy ranker is approximated by a hand-tuned linear blend
// below; the candidate retrieval / diversity passes carry over directly
// so a learned model can drop in later.
//
// 1. CANDIDATE RETRIEVAL — six sources, three per entity type, each
//    capped:
//   - cand_in_network_track       — tracks uploaded in the last 14 days
//     by users I follow. Capped at 200.
//   - cand_trending_track         — top week-trending tracks
//     (track_trending_scores). Mirrors GET /tracks/trending. Capped at 100.
//   - cand_underground_track      — week-trending tracks whose owner has
//     < 1500 follower & following count
//     (mirrors GET /tracks/trending/underground). Capped at 50.
//   - cand_in_network_playlist    — playlists/albums created in the last
//     14 days by users I follow. Capped at 30.
//   - cand_trending_playlist      — top week-trending playlists/albums
//     (playlist_trending_scores). Mirrors GET /playlists/trending.
//     Capped at 30.
//   - cand_underground_playlist   — week-trending playlists/albums whose
//     owner has < 1500 follower & following count. Capped at 15.
//
// Per-source caps are smaller for playlists because they're heavier
// objects to hydrate downstream and rarer in the index — flooding the
// pool with them would crowd out tracks under the shared per-artist cap.
//
// 2. RANKING — three light signals combined linearly, then scaled by a
// content-type weight that keeps the feed track-forward:
//
//	recency_score    = exp(-ln(2) * age_hours / 48)
//	                   // 48h half-life: 48h-old → 0.5, 96h → 0.25
//	engagement_score = ln(1 + 3*saves + 2*reposts + 1*plays) / 12
//	                   // saves > reposts > plays, log-compressed.
//	                   // plays = 0 for playlists (no native play counter).
//	social_boost     = 1.0 + min(my_affinity_with_owner / 4, 1)
//	                   // up to ~2x for owners I already engage with often.
//	                   // Affinity is built from saves+reposts of both
//	                   // tracks AND playlists, plus plays of tracks.
//	source_weight    = {in_network: 1.20, trending: 1.00,
//	                    underground: 0.95}
//	content_type_weight = {track: 1.00, playlist/album: 0.60}
//	                   // collections stay in the feed but rank below
//	                   // comparably-scored tracks, targeting roughly one
//	                   // collection per 5-6 tracks.
//	repeat_penalty   = {played in last 7d: 0.25, played in last 14d: 0.50,
//	                    unplayed: 1.00}
//	                   // tracks the user already heard recently stay in
//	                   // the feed but rank below fresh content. Playlists
//	                   // always get 1.00 (no per-user playlist play data).
//
//	final_score = (0.55 * recency_score + 0.45 * engagement_score)
//	              * social_boost * source_weight * content_type_weight
//	              * repeat_penalty
//
// 3. FILTERS — applied once after the union to keep the candidate set
// cheap:
//   - Track liveness: is_delete / is_unlisted / is_available / stem_of
//   - Playlist liveness: is_delete / is_private
//   - Owner liveness: users.is_deactivated / is_available
//   - access_authorities (tracks only): caller's wallet must be on the
//     list, else ungated only (matches the v1_users_feed authed-wallet
//     pattern). Playlists don't carry access_authorities.
//   - already-saved by the path-param user (don't resurface, both
//     track and playlist saves)
//   - the path-param user's own uploads/playlists
//
// 4. DIVERSITY — shared owner-cap of N items per owner across BOTH
// types via row_number() (configurable via max_per_artist; default 3),
// then a Go-side greedy pass that, when the next item shares an owner
// with the previously emitted one, prefers the next different-owner
// candidate within a 5-position lookahead. "Consecutive same-artist
// penalty" without paying for a second ranker.
//
// PERFORMANCE NOTES: follow_set, the affinity sub-selects, and the
// affinity join itself are all bounded — old engagement is weak signal
// of current taste and unbounded scans are what previously took the
// endpoint over the Cloudflare upstream limit for power users
// (see PRs #805 and #806). The trending sources rely on partial indexes
// on track_trending_scores / playlist_trending_scores for the
// pnagD/week/null-genre slice.
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
//   - max_per_artist (default 3,  max 10) — owner cap per page,
//     shared across tracks + playlists.
//   - user_id        (optional): the caller's id. Independent of the
//     path id — used to populate has_current_user_reposted and similar
//     viewer-relative fields on the returned items. Same role it plays
//     on every other /v1/users/{id}/... endpoint. The auth middleware
//     treats this as advisory rather than authoritative on this route.
func (app *ApiServer) v1FeedForYou(c *fiber.Ctx) error {
	var params = GetUsersFeedForYouParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	userId := app.getUserId(c)
	myId := app.getMyId(c)
	authedWallet := app.tryGetAuthedWallet(c)

	// Pull a candidate pool larger than the page size so the diversity
	// cap and the consecutive-same-artist pass have headroom to reorder.
	const candidatePoolSize = 200

	sql := `
	WITH
	-- Cap to the 500 most-recently-followed users. A power user with
	-- thousands of follows pulls a huge hash table here that then has to
	-- join against every recent upload to find in-network candidates,
	-- so the planner can stall. Recent follows are a better signal of
	-- current taste anyway.
	follow_set AS (
		SELECT followee_user_id AS user_id
		FROM follows
		WHERE follower_user_id = @userId
		  AND is_current = true
		  AND is_delete = false
		ORDER BY created_at DESC
		LIMIT 500
	),
	my_saved_tracks AS (
		SELECT save_item_id AS track_id
		FROM saves
		WHERE user_id = @userId
		  AND save_type = 'track'
		  AND is_current = true
		  AND is_delete = false
	),
	my_saved_playlists AS (
		SELECT save_item_id AS playlist_id
		FROM saves
		WHERE user_id = @userId
		  AND save_type IN ('playlist', 'album')
		  AND is_current = true
		  AND is_delete = false
	),
	-- Tracks I've played in the last 14 days, with the most recent play
	-- time. Used for the repeat_penalty multiplier so already-heard
	-- tracks rank below fresh content without being filtered out.
	--
	-- Bounded the same way as the affinity play sub-select: a heavy
	-- listener can have hundreds of thousands of play rows, so scan only
	-- the most recent slice before applying the 14-day window.
	my_recent_plays AS (
		SELECT track_id, MAX(created_at) AS last_played_at
		FROM (
			SELECT play_item_id AS track_id, created_at
			FROM plays
			WHERE user_id = @userId
			ORDER BY created_at DESC
			LIMIT 2000
		) p
		WHERE created_at >= NOW() - INTERVAL '14 days'
		GROUP BY track_id
	),
	-- Per-owner engagement strength: saves+reposts of any of their
	-- tracks/playlists by me, plus plays of their tracks. Used for the
	-- social_boost multiplier.
	--
	-- Each sub-select is bounded by recency: a heavy listener can have
	-- hundreds of thousands of play rows, and the unbounded union forces
	-- a full scan on every request. The inner joins are further
	-- restricted to entities created in the last 90 days so old uploads
	-- can't pull this CTE wide. Old engagement is a weak signal of
	-- current taste anyway.
	my_artist_affinity AS (
		SELECT owner_id AS artist_id,
		       LN(1 + COUNT(*)) AS affinity
		FROM (
			SELECT t.owner_id
			FROM (
				SELECT save_item_id AS track_id FROM saves
				WHERE user_id = @userId AND save_type = 'track'
				  AND is_current = true AND is_delete = false
				ORDER BY created_at DESC
				LIMIT 200
			) s
			JOIN tracks t ON t.track_id = s.track_id
			            AND t.created_at >= NOW() - INTERVAL '90 days'

			UNION ALL

			SELECT t.owner_id
			FROM (
				SELECT repost_item_id AS track_id FROM reposts
				WHERE user_id = @userId AND repost_type = 'track'
				  AND is_current = true AND is_delete = false
				ORDER BY created_at DESC
				LIMIT 200
			) r
			JOIN tracks t ON t.track_id = r.track_id
			            AND t.created_at >= NOW() - INTERVAL '90 days'

			UNION ALL

			SELECT t.owner_id
			FROM (
				SELECT play_item_id AS track_id FROM plays
				WHERE user_id = @userId
				ORDER BY created_at DESC
				LIMIT 500
			) p
			JOIN tracks t ON t.track_id = p.track_id
			            AND t.created_at >= NOW() - INTERVAL '90 days'

			UNION ALL

			SELECT pl.playlist_owner_id AS owner_id
			FROM (
				SELECT save_item_id AS playlist_id FROM saves
				WHERE user_id = @userId AND save_type IN ('playlist', 'album')
				  AND is_current = true AND is_delete = false
				ORDER BY created_at DESC
				LIMIT 200
			) sp
			JOIN playlists pl ON pl.playlist_id = sp.playlist_id
			               AND pl.created_at >= NOW() - INTERVAL '90 days'

			UNION ALL

			SELECT pl.playlist_owner_id AS owner_id
			FROM (
				SELECT repost_item_id AS playlist_id FROM reposts
				WHERE user_id = @userId AND repost_type IN ('playlist', 'album')
				  AND is_current = true AND is_delete = false
				ORDER BY created_at DESC
				LIMIT 200
			) rp
			JOIN playlists pl ON pl.playlist_id = rp.playlist_id
			               AND pl.created_at >= NOW() - INTERVAL '90 days'
		) eng
		GROUP BY owner_id
	),
	-- Source 1a: in-network (followed-creator) recent track uploads.
	cand_in_network_track AS (
		SELECT t.track_id AS entity_id,
		       'track'::text AS entity_type,
		       'in_network'::text AS source
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
	-- Source 2a: weekly trending tracks.
	cand_trending_track AS (
		SELECT tts.track_id AS entity_id,
		       'track'::text AS entity_type,
		       'trending'::text AS source
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
	-- Source 3a: underground trending tracks (sub-1500 follower & following artists).
	cand_underground_track AS (
		SELECT tts.track_id AS entity_id,
		       'track'::text AS entity_type,
		       'underground'::text AS source
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
	-- Source 1b: in-network recent playlist/album creations.
	cand_in_network_playlist AS (
		SELECT p.playlist_id AS entity_id,
		       'playlist'::text AS entity_type,
		       'in_network'::text AS source
		FROM playlists p
		JOIN follow_set f ON f.user_id = p.playlist_owner_id
		WHERE p.is_current = true
		  AND p.is_delete = false
		  AND p.is_private = false
		  AND p.created_at >= NOW() - INTERVAL '14 days'
		ORDER BY p.created_at DESC
		LIMIT 30
	),
	-- Source 2b: weekly trending playlists/albums.
	cand_trending_playlist AS (
		SELECT pts.playlist_id AS entity_id,
		       'playlist'::text AS entity_type,
		       'trending'::text AS source
		FROM playlist_trending_scores pts
		JOIN playlists p ON p.playlist_id = pts.playlist_id
		            AND p.is_current = true
		            AND p.is_delete = false
		            AND p.is_private = false
		WHERE pts.type = 'PLAYLISTS'
		  AND pts.version = 'pnagD'
		  AND pts.time_range = 'week'
		ORDER BY pts.score DESC, pts.playlist_id DESC
		LIMIT 30
	),
	-- Source 3b: underground trending playlists/albums.
	cand_underground_playlist AS (
		SELECT pts.playlist_id AS entity_id,
		       'playlist'::text AS entity_type,
		       'underground'::text AS source
		FROM playlist_trending_scores pts
		JOIN playlists p ON p.playlist_id = pts.playlist_id
		            AND p.is_current = true
		            AND p.is_delete = false
		            AND p.is_private = false
		JOIN aggregate_user au ON au.user_id = p.playlist_owner_id
		WHERE pts.type = 'PLAYLISTS'
		  AND pts.version = 'pnagD'
		  AND pts.time_range = 'week'
		  AND au.follower_count < 1500
		  AND au.following_count < 1500
		ORDER BY pts.score DESC, pts.playlist_id DESC
		LIMIT 15
	),
	-- One row per (entity_type, entity_id). DISTINCT ON keeps the
	-- strongest (lowest-prio) source so an in-network item that's also
	-- trending keeps the in_network weight rather than the lower
	-- trending weight.
	candidates AS (
		SELECT DISTINCT ON (entity_type, entity_id)
		       entity_type, entity_id, source
		FROM (
			SELECT entity_type, entity_id, source, 1 AS prio FROM cand_in_network_track
			UNION ALL
			SELECT entity_type, entity_id, source, 2 AS prio FROM cand_trending_track
			UNION ALL
			SELECT entity_type, entity_id, source, 3 AS prio FROM cand_underground_track
			UNION ALL
			SELECT entity_type, entity_id, source, 1 AS prio FROM cand_in_network_playlist
			UNION ALL
			SELECT entity_type, entity_id, source, 2 AS prio FROM cand_trending_playlist
			UNION ALL
			SELECT entity_type, entity_id, source, 3 AS prio FROM cand_underground_playlist
		) u
		ORDER BY entity_type, entity_id, prio
	),
	filtered_tracks AS (
		SELECT
			'track'::text                AS entity_type,
			c.entity_id                  AS entity_id,
			t.owner_id                   AS owner_id,
			t.created_at                 AS created_at,
			c.source                     AS source,
			COALESCE(at.save_count, 0)   AS save_count,
			COALESCE(at.repost_count, 0) AS repost_count,
			COALESCE(ap.count, 0)        AS play_count,
			COALESCE(maa.affinity, 0)    AS affinity,
			mrp.last_played_at           AS last_played_at
		FROM candidates c
		JOIN tracks t ON t.track_id = c.entity_id
		JOIN users  u ON u.user_id  = t.owner_id
		LEFT JOIN aggregate_track at  ON at.track_id     = c.entity_id
		LEFT JOIN aggregate_plays ap  ON ap.play_item_id = c.entity_id
		LEFT JOIN my_artist_affinity maa ON maa.artist_id = t.owner_id
		LEFT JOIN my_recent_plays mrp ON mrp.track_id = c.entity_id
		WHERE c.entity_type = 'track'
		  AND t.is_current   = true
		  AND t.is_delete    = false
		  AND t.is_unlisted  = false
		  AND t.is_available = true
		  AND t.stem_of IS NULL
		  AND u.is_current     = true
		  AND u.is_deactivated = false
		  AND u.is_available   = true
		  AND (t.access_authorities IS NULL
		       OR (COALESCE(@authed_wallet, '') <> ''
		           AND EXISTS (
		               SELECT 1 FROM unnest(t.access_authorities) aa
		               WHERE lower(aa) = lower(@authed_wallet)
		           )))
		  AND NOT EXISTS (
		      SELECT 1 FROM my_saved_tracks ms WHERE ms.track_id = c.entity_id
		  )
		  AND t.owner_id <> @userId
	),
	filtered_playlists AS (
		SELECT
			'playlist'::text             AS entity_type,
			c.entity_id                  AS entity_id,
			p.playlist_owner_id          AS owner_id,
			p.created_at                 AS created_at,
			c.source                     AS source,
			COALESCE(ap.save_count, 0)   AS save_count,
			COALESCE(ap.repost_count, 0) AS repost_count,
			0                            AS play_count,
			COALESCE(maa.affinity, 0)    AS affinity,
			NULL::timestamp              AS last_played_at
		FROM candidates c
		JOIN playlists p ON p.playlist_id = c.entity_id
		JOIN users     u ON u.user_id     = p.playlist_owner_id
		LEFT JOIN aggregate_playlist ap   ON ap.playlist_id = c.entity_id
		LEFT JOIN my_artist_affinity maa  ON maa.artist_id  = p.playlist_owner_id
		WHERE c.entity_type = 'playlist'
		  AND p.is_current = true
		  AND p.is_delete  = false
		  AND p.is_private = false
		  AND u.is_current     = true
		  AND u.is_deactivated = false
		  AND u.is_available   = true
		  AND NOT EXISTS (
		      SELECT 1 FROM my_saved_playlists ms WHERE ms.playlist_id = c.entity_id
		  )
		  AND p.playlist_owner_id <> @userId
	),
	filtered AS (
		SELECT entity_type, entity_id, owner_id, created_at, source,
		       save_count, repost_count, play_count, affinity, last_played_at
		FROM filtered_tracks
		UNION ALL
		SELECT entity_type, entity_id, owner_id, created_at, source,
		       save_count, repost_count, play_count, affinity, last_played_at
		FROM filtered_playlists
	),
	scored AS (
		SELECT
			f.entity_type,
			f.entity_id,
			f.owner_id,
			f.created_at,
			EXP(GREATEST(
				-LN(2.0::double precision) * GREATEST(EXTRACT(EPOCH FROM (NOW() - f.created_at))::double precision / 3600.0, 0.0) / 48.0,
				-700.0
			))
				AS recency_score,
			LN(1 + 3 * f.save_count + 2 * f.repost_count + f.play_count) / 12.0
				AS engagement_score,
			1.0 + LEAST(f.affinity / 4.0, 1.0) AS social_boost,
			CASE f.source
				WHEN 'in_network'  THEN 1.20
				WHEN 'trending'    THEN 1.00
				WHEN 'underground' THEN 0.95
				ELSE 1.00
			END AS source_weight,
			CASE f.entity_type
				WHEN 'track' THEN 1.00
				ELSE 0.60
			END AS content_type_weight,
			CASE
				WHEN f.last_played_at >= NOW() - INTERVAL '7 days'  THEN 0.25
				WHEN f.last_played_at >= NOW() - INTERVAL '14 days' THEN 0.50
				ELSE 1.00
			END AS repeat_penalty
		FROM filtered f
	),
	final_scored AS (
		SELECT
			entity_type,
			entity_id,
			owner_id,
			created_at,
			(0.55 * recency_score + 0.45 * engagement_score)
				* social_boost * source_weight * content_type_weight
				* repeat_penalty AS score
		FROM scored
	),
	capped AS (
		SELECT entity_type, entity_id, owner_id, created_at, score,
		       ROW_NUMBER() OVER (PARTITION BY owner_id ORDER BY score DESC, entity_id DESC)
		         AS rn_artist
		FROM final_scored
	)
	SELECT entity_type, entity_id, owner_id, created_at
	FROM capped
	WHERE rn_artist <= @maxPerArtist
	ORDER BY score DESC, entity_id DESC
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
		EntityType string
		EntityID   int32
		OwnerID    int32
		CreatedAt  time.Time
	}
	pool, err := pgx.CollectRows(rows, pgx.RowToStructByPos[ranked])
	if err != nil {
		return err
	}

	// Greedy diversity pass: keep the global rank order, but if the next
	// item shares an owner with the one just emitted, prefer the next
	// different-owner candidate within a small lookahead. Soft penalty
	// on consecutive-same-artist runs without computing a second ranker.
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

	start := params.Offset
	if start > len(ordered) {
		start = len(ordered)
	}
	end := start + params.Limit
	if end > len(ordered) {
		end = len(ordered)
	}
	page := ordered[start:end]

	trackIds := []int32{}
	playlistIds := []int32{}
	for _, r := range page {
		if r.EntityType == "track" {
			trackIds = append(trackIds, r.EntityID)
		} else {
			playlistIds = append(playlistIds, r.EntityID)
		}
	}

	loaded, err := app.queries.Parallel(c.Context(), dbv1.ParallelParams{
		TrackIds:     trackIds,
		PlaylistIds:  playlistIds,
		MyID:         myId,
		AuthedWallet: authedWallet,
	})
	if err != nil {
		return err
	}

	type FeedItem struct {
		EntityType string    `json:"type"`
		CreatedAt  time.Time `json:"timestamp"`
		Item       any       `json:"item"`
	}

	items := make([]FeedItem, 0, len(page))
	for _, r := range page {
		if r.EntityType == "track" {
			if t, ok := loaded.TrackMap[r.EntityID]; ok {
				items = append(items, FeedItem{
					EntityType: "track",
					CreatedAt:  r.CreatedAt,
					Item:       t,
				})
			}
		} else {
			if p, ok := loaded.PlaylistMap[r.EntityID]; ok {
				items = append(items, FeedItem{
					EntityType: "playlist",
					CreatedAt:  r.CreatedAt,
					Item:       p,
				})
			}
		}
	}

	return c.JSON(fiber.Map{
		"data": items,
	})
}
