package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersFeedForYouParams struct {
	Limit  int `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0,max=200"`
}

// v1UsersFeedForYou returns a personalized "For You" track feed for the
// user identified in the path. It replaces the client-side blend in
// apps/packages/common/src/api/tan-query/lineups/useForYouFeed.ts: four
// candidate streams (recommended, following originals, weekly trending,
// underground trending) interleaved with a fixed 10-slot pattern,
//
//	pos: [R R R F R T R F U R]
//
// where R=recommended, F=following, T=trending, U=underground. That gives
// 60% recommended (50% baseline + 10% filler when other sources thin
// out), 20% following, 10% trending, 10% underground. When a slot's
// preferred source is exhausted we fall through to the next source in
// priority order recommended → following → trending → underground.
//
// Each source is independently capped at 200 candidates. The union is
// deduped by track_id and the caller's already-saved tracks are filtered
// out at the SQL level (the client filters them on the way out;
// filtering early avoids burning candidates on already-saved tracks).
// Pagination (limit/offset) is applied to the composed list, so pages
// are stable as long as the underlying source ranks haven't shifted.
//
// Source SQL mirrors the underlying single-source endpoints:
//   - recommended: v1_users_recommended_tracks.go (top tracks from the
//     user's top-played genres, excluding tracks they've played). Uses
//     score-DESC ordering instead of random() so pagination is stable.
//   - following:   v1_users_feed.go w/ filter=original (track originals
//     from artists the user follows, last year)
//   - trending:    v1_tracks_trending.go (week)
//   - underground: v1_tracks_trending_underground.go (week, sub-1500
//     follower & following artists)
//
// Liveness, gating, and unlisted/deleted filters mirror the patterns
// used in those files and in v1_events_remix_contests.go.
func (app *ApiServer) v1UsersFeedForYou(c *fiber.Ctx) error {
	var params GetUsersFeedForYouParams
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	userId := app.getUserId(c)
	myId := app.getMyId(c)
	authedWallet := app.tryGetAuthedWallet(c)

	// Pull a generous pool from each source so the interleave has room
	// to dedupe even at large offsets.
	const perSourceLimit = 200

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
	played_tracks AS (
		SELECT DISTINCT play_item_id
		FROM plays
		WHERE user_id = @userId
	),
	top_genres AS (
		SELECT t.genre
		FROM played_tracks pt
		JOIN tracks t ON t.track_id = pt.play_item_id
		WHERE t.genre IS NOT NULL
		GROUP BY t.genre
		ORDER BY COUNT(*) DESC
		LIMIT 5
	),
	cand_recommended AS (
		SELECT
			tts.track_id,
			'recommended'::text AS source,
			ROW_NUMBER() OVER (ORDER BY tts.score DESC, tts.track_id DESC) AS rn
		FROM track_trending_scores tts
		JOIN top_genres tg ON tg.genre = tts.genre
		JOIN tracks t ON t.track_id = tts.track_id
		JOIN users u ON u.user_id = t.owner_id
		WHERE tts.type = 'TRACKS'
		  AND tts.version = 'pnagD'
		  AND tts.time_range = 'week'
		  AND t.is_delete = false
		  AND t.is_unlisted = false
		  AND t.is_available = true
		  AND t.stem_of IS NULL
		  AND u.is_deactivated = false
		  AND (t.access_authorities IS NULL
			OR (COALESCE(@authedWallet, '') <> ''
				AND EXISTS (SELECT 1 FROM unnest(t.access_authorities) aa
							WHERE lower(aa) = lower(@authedWallet))))
		  AND NOT EXISTS (SELECT 1 FROM played_tracks pt WHERE pt.play_item_id = tts.track_id)
		  AND NOT EXISTS (SELECT 1 FROM my_saved_tracks ms WHERE ms.track_id = tts.track_id)
		ORDER BY tts.score DESC, tts.track_id DESC
		LIMIT @perSource
	),
	cand_following AS (
		SELECT
			t.track_id,
			'following'::text AS source,
			ROW_NUMBER() OVER (ORDER BY t.created_at DESC, t.track_id DESC) AS rn
		FROM tracks t
		JOIN follow_set fs ON fs.user_id = t.owner_id
		JOIN users u ON u.user_id = t.owner_id
		WHERE t.is_delete = false
		  AND t.is_unlisted = false
		  AND t.is_available = true
		  AND t.stem_of IS NULL
		  AND t.created_at >= NOW() - INTERVAL '1 YEAR'
		  AND u.is_deactivated = false
		  AND (t.access_authorities IS NULL
			OR (COALESCE(@authedWallet, '') <> ''
				AND EXISTS (SELECT 1 FROM unnest(t.access_authorities) aa
							WHERE lower(aa) = lower(@authedWallet))))
		  AND NOT EXISTS (SELECT 1 FROM my_saved_tracks ms WHERE ms.track_id = t.track_id)
		ORDER BY t.created_at DESC, t.track_id DESC
		LIMIT @perSource
	),
	cand_trending AS (
		SELECT
			tts.track_id,
			'trending'::text AS source,
			ROW_NUMBER() OVER (ORDER BY tts.score DESC, tts.track_id DESC) AS rn
		FROM track_trending_scores tts
		JOIN tracks t ON t.track_id = tts.track_id
		JOIN users u ON u.user_id = t.owner_id
		WHERE tts.type = 'TRACKS'
		  AND tts.version = 'pnagD'
		  AND tts.time_range = 'week'
		  AND (tts.genre IS NULL OR tts.genre = '')
		  AND t.is_delete = false
		  AND t.is_unlisted = false
		  AND t.is_available = true
		  AND u.is_deactivated = false
		  AND NOT EXISTS (SELECT 1 FROM my_saved_tracks ms WHERE ms.track_id = tts.track_id)
		ORDER BY tts.score DESC, tts.track_id DESC
		LIMIT @perSource
	),
	cand_underground AS (
		SELECT
			tts.track_id,
			'underground'::text AS source,
			ROW_NUMBER() OVER (ORDER BY tts.score DESC, tts.track_id DESC) AS rn
		FROM track_trending_scores tts
		JOIN tracks t ON t.track_id = tts.track_id
		JOIN users u ON u.user_id = t.owner_id
		JOIN aggregate_user au ON au.user_id = t.owner_id
		WHERE tts.type = 'TRACKS'
		  AND tts.version = 'pnagD'
		  AND tts.time_range = 'week'
		  AND (tts.genre IS NULL OR tts.genre = '')
		  AND t.is_delete = false
		  AND t.is_unlisted = false
		  AND t.is_available = true
		  AND u.is_deactivated = false
		  AND au.follower_count < 1500
		  AND au.following_count < 1500
		  AND NOT EXISTS (SELECT 1 FROM my_saved_tracks ms WHERE ms.track_id = tts.track_id)
		ORDER BY tts.score DESC, tts.track_id DESC
		LIMIT @perSource
	)
	SELECT track_id, source, rn FROM cand_recommended
	UNION ALL
	SELECT track_id, source, rn FROM cand_following
	UNION ALL
	SELECT track_id, source, rn FROM cand_trending
	UNION ALL
	SELECT track_id, source, rn FROM cand_underground
	ORDER BY source, rn
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId":       userId,
		"perSource":    perSourceLimit,
		"authedWallet": authedWallet,
	})
	if err != nil {
		return err
	}

	type candRow struct {
		TrackID int32  `db:"track_id"`
		Source  string `db:"source"`
		Rn      int32  `db:"rn"`
	}
	cands, err := pgx.CollectRows(rows, pgx.RowToStructByName[candRow])
	if err != nil {
		return err
	}

	bySrc := map[string][]int32{
		"recommended": {},
		"following":   {},
		"trending":    {},
		"underground": {},
	}
	for _, r := range cands {
		bySrc[r.Source] = append(bySrc[r.Source], r.TrackID)
	}

	// Build the composed list up to the page boundary so offset/limit are
	// deterministic.
	target := params.Offset + params.Limit
	composed := blendForYouSources(bySrc, target)

	start := params.Offset
	if start > len(composed) {
		start = len(composed)
	}
	end := start + params.Limit
	if end > len(composed) {
		end = len(composed)
	}
	pageIds := composed[start:end]

	tracks, err := app.queries.Tracks(c.Context(), dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:          pageIds,
			MyID:         myId,
			AuthedWallet: authedWallet,
		},
	})
	if err != nil {
		return err
	}

	byId := make(map[int32]dbv1.Track, len(tracks))
	for _, t := range tracks {
		byId[t.TrackID] = t
	}
	sorted := make([]dbv1.Track, 0, len(pageIds))
	for _, id := range pageIds {
		if t, ok := byId[id]; ok {
			sorted = append(sorted, t)
		}
	}

	return v1TracksResponse(c, sorted)
}

// forYouSlotPattern is the 10-slot interleave used by useForYouFeed.ts in
// apps. Six recommended, two following, one trending, one underground.
var forYouSlotPattern = []string{
	"recommended", "recommended", "recommended",
	"following", "recommended", "trending",
	"recommended", "following", "underground",
	"recommended",
}

// forYouFallbackOrder is the priority order to fall through to when a
// slot's preferred source has run out of candidates.
var forYouFallbackOrder = []string{"recommended", "following", "trending", "underground"}

// blendForYouSources interleaves four ranked candidate lists using the
// slot pattern above. Dedupes globally by track_id; falls through to the
// next source in priority order when the slot's preferred source is
// exhausted. Returns up to `target` track ids.
func blendForYouSources(bySrc map[string][]int32, target int) []int32 {
	cursors := map[string]int{
		"recommended": 0, "following": 0, "trending": 0, "underground": 0,
	}
	seen := map[int32]bool{}
	composed := make([]int32, 0, target)

	tryTake := func(src string) (int32, bool) {
		list := bySrc[src]
		for cursors[src] < len(list) {
			id := list[cursors[src]]
			cursors[src]++
			if seen[id] {
				continue
			}
			seen[id] = true
			return id, true
		}
		return 0, false
	}

	slot := 0
	for len(composed) < target {
		preferred := forYouSlotPattern[slot%len(forYouSlotPattern)]
		// Build the per-slot probe order: preferred first, then the rest
		// of the fallback in declared order, skipping the duplicate.
		order := make([]string, 0, len(forYouFallbackOrder))
		order = append(order, preferred)
		for _, k := range forYouFallbackOrder {
			if k != preferred {
				order = append(order, k)
			}
		}

		picked := int32(-1)
		for _, src := range order {
			if id, ok := tryTake(src); ok {
				picked = id
				break
			}
		}
		if picked == -1 {
			break
		}
		composed = append(composed, picked)
		slot++
	}
	return composed
}
