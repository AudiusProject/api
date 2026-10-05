package api

import (
	"context"
	"fmt"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/weeklyrotation"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersWeeklyRotationParams struct {
	Limit int `query:"limit" default:"30" validate:"min=1,max=50"`
}

const (
	// Tracks older than this are excluded. Old tracks that never found an
	// audience are rarely good discoveries.
	weeklyRotationMaxAgeDays = 365

	// Week-seeded jitter band. Candidate scores are tightly clustered, so
	// without a per-week perturbation the same user would get a near-identical
	// mix every week. +/-15% reorders comparable candidates without letting a
	// weak track outrank a clearly better one.
	weeklyRotationJitterFloor = 0.85
	weeklyRotationJitterRange = 0.30

	// Every period's mix is stored at this size (the max limit), so any
	// limit is a prefix of the same list.
	weeklyRotationStoredSize = 50

	// Tracks in the first weeklyRotationSeenSize slots (what clients show)
	// of the last weeklyRotationLookbackPeriods mixes are not repeated.
	// Without this an unplayed track that keeps trending comes back every
	// week.
	weeklyRotationSeenSize        = 30
	weeklyRotationLookbackPeriods = 2

	// Long-form sets and DJ mixes stay in the mix but never open it.
	weeklyRotationLongFormSeconds = 15 * 60
)

/*
Returns a taste-matched track mix that is stable for the rotation period (the
"Weekly Rotation" surface). The period rolls over on Wednesday 00:00 UTC, see
weeklyrotation.RolloverOffsetDays.

Differences from GET /v1/users/{id}/feed/for-you:
  - The mix is fixed for the period instead of re-ranked on every load.
  - Followed artists are demoted instead of boosted.
  - Played and saved tracks are excluded instead of soft-penalized.

STABILITY. The first request in a period computes the mix and stores it in
weekly_rotation_mixes keyed by (user_id, period_start); every later request
in that period, on any node, returns the stored list. Tracks deleted or
unlisted since are dropped on read. Without a write pool (some local setups)
the mix is computed on every cache miss.

The query is deterministic given (user_id, iso_year, iso_week). The
week-to-week variation comes from week_seed, a hash of (track_id, user_id,
year, week); nothing uses random(). The listener's history (plays, saves,
reposts, follows) is read as of the period start.

NO REPEATS. Tracks shown in the previous weeklyRotationLookbackPeriods stored
mixes are excluded. Trending moves slowly, so without this a track the
listener skipped kept its slot week after week.

SCORING.

	quality_score  = ln(1 + 3*saves + 2*reposts + 1*plays) / 12
	                 // same engagement blend as For You.
	genre_affinity = 0.85 + 0.45 * min(genre_share / 0.30, 1)
	                 // genre_share is the fraction of my recent plays in
	                 // the track's genre (same as For You).
	discovery_wt   = {not followed, no artist affinity: 1.25,
	                  not followed, artist affinity:    1.00,
	                  followed:                         0.70}
	source_weight  = {underground: 1.15, trending: 1.00}
	week_seed      = 0.85 + 0.30 * hash01(track_id, user_id, year, week)

	final_score = quality_score * genre_affinity * discovery_wt
	              * source_weight * week_seed

FILTERS. Track liveness (is_delete / is_unlisted / is_available / stem_of),
owner liveness (is_deactivated / is_available), gated tracks, own uploads,
anything played, anything saved, anything in a recent mix, and anything
older than weeklyRotationMaxAgeDays.

DIVERSITY. One track per artist (For You allows 3). The first track is never
longer than weeklyRotationLongFormSeconds, so the mix doesn't open with an
hour-long set.

Path:
  - id (required): the user being personalized for. Resolved by
    requireUserIdMiddleware.

Query params:
  - limit (default 30, max 50)
  - user_id (optional): the caller, for viewer-relative fields on the
    returned tracks. Independent of the path id, same as elsewhere.
*/
func (app *ApiServer) v1UsersWeeklyRotation(c *fiber.Ctx) error {
	params := GetUsersWeeklyRotationParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	userId := app.getUserId(c)
	myId := app.getMyId(c)

	year, week := weeklyrotation.Period(time.Now())

	trackIds, err := app.getWeeklyRotationTrackIds(
		c.Context(),
		userId,
		year,
		week,
		params.Limit,
	)
	if err != nil {
		return err
	}

	// Tracks preserves the order of Ids.
	tracks, err := app.queries.Tracks(c.Context(), dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:          trackIds,
			MyID:         myId,
			AuthedWallet: app.tryGetAuthedWallet(c),
		},
	})
	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}

func (app *ApiServer) getWeeklyRotationTrackIds(
	ctx context.Context,
	userId int32,
	year int,
	week int,
	limit int,
) ([]int32, error) {
	cacheKey := fmt.Sprintf("weekly_rotation:%d:%d:%d", userId, year, week)
	ids, ok := app.weeklyRotationCache.Get(cacheKey)
	if !ok {
		var err error
		ids, err = app.loadWeeklyRotationMix(ctx, userId, year, week)
		if err != nil {
			return nil, err
		}
		app.weeklyRotationCache.Set(cacheKey, ids)
	}
	if len(ids) > limit {
		ids = ids[:limit]
	}
	return ids, nil
}

// loadWeeklyRotationMix returns the stored mix for the period, computing and
// storing it on first request. See STABILITY in the handler doc.
func (app *ApiServer) loadWeeklyRotationMix(
	ctx context.Context,
	userId int32,
	year int,
	week int,
) ([]int32, error) {
	if app.writePool == nil {
		return app.computeWeeklyRotationMix(ctx, userId, year, week, nil)
	}

	periodStart := weeklyrotation.PeriodStart(year, week)

	var stored []int32
	err := app.writePool.QueryRow(ctx, `
		SELECT track_ids FROM weekly_rotation_mixes
		WHERE user_id = $1 AND period_start = $2
	`, userId, periodStart).Scan(&stored)
	if err == nil {
		return app.filterLiveWeeklyRotationTracks(ctx, stored)
	}
	if err != pgx.ErrNoRows {
		return nil, err
	}

	var recent []int32
	err = app.writePool.QueryRow(ctx, `
		SELECT COALESCE(ARRAY_AGG(DISTINCT id), '{}')
		FROM weekly_rotation_mixes m, UNNEST(m.track_ids[1:$4]) AS id
		WHERE m.user_id = $1
		  AND m.period_start >= $2::date - $3::int * 7
		  AND m.period_start < $2::date
	`, userId, periodStart, weeklyRotationLookbackPeriods, weeklyRotationSeenSize).Scan(&recent)
	if err != nil {
		return nil, err
	}

	ids, err := app.computeWeeklyRotationMix(ctx, userId, year, week, recent)
	if err != nil || len(ids) == 0 {
		return ids, err
	}

	// Another node may have stored this period's mix first. Theirs wins so
	// every node serves the same list.
	err = app.writePool.QueryRow(ctx, `
		INSERT INTO weekly_rotation_mixes (user_id, period_start, track_ids)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, period_start)
		DO UPDATE SET track_ids = weekly_rotation_mixes.track_ids
		RETURNING track_ids
	`, userId, periodStart, ids).Scan(&stored)
	if err != nil {
		return nil, err
	}
	return stored, nil
}

// filterLiveWeeklyRotationTracks drops tracks from a stored mix that have
// been deleted or unlisted, or whose owner was deactivated, since it was
// stored. Order is preserved.
func (app *ApiServer) filterLiveWeeklyRotationTracks(
	ctx context.Context,
	ids []int32,
) ([]int32, error) {
	rows, err := app.pool.Query(ctx, `
		SELECT t.track_id
		FROM UNNEST($1::int[]) WITH ORDINALITY AS m(track_id, ord)
		JOIN tracks t ON t.track_id = m.track_id AND t.is_current = true
		JOIN users u  ON u.user_id = t.owner_id AND u.is_current = true
		WHERE t.is_delete = false
		  AND t.is_unlisted = false
		  AND t.is_available = true
		  AND u.is_deactivated = false
		  AND u.is_available = true
		ORDER BY m.ord
	`, ids)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowTo[int32])
}

// computeWeeklyRotationMix ranks the mix for a period, leaving out
// excludeTrackIds. Returns up to weeklyRotationStoredSize track ids.
func (app *ApiServer) computeWeeklyRotationMix(
	ctx context.Context,
	userId int32,
	year int,
	week int,
	excludeTrackIds []int32,
) ([]int32, error) {
	if excludeTrackIds == nil {
		excludeTrackIds = []int32{}
	}

	sql := `
	WITH
	-- Tracks the listener has already heard, excluded outright. Capped at the
	-- latest 10k plays so heavy listeners stay under the upstream timeout
	-- (see #805, #806).
	--
	-- Every history CTE is cut off at @periodStart so listening to this
	-- period's mix doesn't change it. See STABILITY in the handler doc.
	my_played AS (
		SELECT DISTINCT play_item_id AS track_id
		FROM (
			SELECT play_item_id
			FROM plays
			WHERE user_id = @userId
			  AND created_at < @periodStart
			ORDER BY created_at DESC
			LIMIT 10000
		) p
	),
	my_saved AS (
		SELECT save_item_id AS track_id
		FROM saves
		WHERE user_id = @userId
		  AND save_type = 'track'
		  AND is_current = true
		  AND is_delete = false
		  AND created_at < @periodStart
	),
	-- Capped like the For You feed's follow_set: thousands of follows
	-- otherwise stall the planner on the join below.
	follow_set AS (
		SELECT followee_user_id AS user_id
		FROM follows
		WHERE follower_user_id = @userId
		  AND is_current = true
		  AND is_delete = false
		  AND created_at < @periodStart
		ORDER BY created_at DESC
		LIMIT 500
	),
	-- Genre mix of recent listening (same as the For You feed).
	my_genre_affinity AS (
		SELECT t.genre,
		       COUNT(*)::double precision / SUM(COUNT(*)) OVER () AS share
		FROM (
			SELECT play_item_id AS track_id
			FROM plays
			WHERE user_id = @userId
			  AND created_at < @periodStart
			ORDER BY created_at DESC
			LIMIT 1000
		) p
		JOIN tracks t ON t.track_id = p.track_id
		WHERE t.genre IS NOT NULL AND t.genre <> ''
		GROUP BY t.genre
	),
	-- Artists the listener already saves or reposts. Demoted below, since
	-- they aren't discoveries. Bounded by recency like the For You CTE.
	my_artist_affinity AS (
		SELECT owner_id AS artist_id
		FROM (
			SELECT t.owner_id
			FROM (
				SELECT save_item_id AS track_id FROM saves
				WHERE user_id = @userId AND save_type = 'track'
				  AND is_current = true AND is_delete = false
				  AND created_at < @periodStart
				ORDER BY created_at DESC
				LIMIT 200
			) s
			JOIN tracks t ON t.track_id = s.track_id

			UNION ALL

			SELECT t.owner_id
			FROM (
				SELECT repost_item_id AS track_id FROM reposts
				WHERE user_id = @userId AND repost_type = 'track'
				  AND is_current = true AND is_delete = false
				  AND created_at < @periodStart
				ORDER BY created_at DESC
				LIMIT 200
			) r
			JOIN tracks t ON t.track_id = r.track_id
		) eng
		GROUP BY owner_id
	),
	-- Source 1: weekly trending tracks.
	--
	-- No genre predicate: rows with a genre are the live trending list, and
	-- rows with a null/empty genre are years-old leftovers. Matches
	-- GET /tracks/trending.
	cand_trending AS (
		SELECT tts.track_id, 'trending'::text AS source
		FROM track_trending_scores tts
		WHERE tts.type = 'TRACKS'
		  AND tts.version = 'pnagD'
		  AND tts.time_range = 'week'
		ORDER BY tts.score DESC, tts.track_id DESC
		LIMIT 400
	),
	-- Source 2: the same trending slice restricted to small creators, like
	-- GET /tracks/trending/underground. Upweighted below.
	cand_underground AS (
		SELECT tts.track_id, 'underground'::text AS source
		FROM track_trending_scores tts
		JOIN tracks t ON t.track_id = tts.track_id
		JOIN aggregate_user au ON au.user_id = t.owner_id
		WHERE tts.type = 'TRACKS'
		  AND tts.version = 'pnagD'
		  AND tts.time_range = 'week'
		  AND au.follower_count < 1500
		  AND au.following_count < 1500
		ORDER BY tts.score DESC, tts.track_id DESC
		LIMIT 300
	),
	candidates AS (
		SELECT track_id, source FROM cand_underground
		UNION ALL
		SELECT track_id, source FROM cand_trending
	),
	-- One row per track. A track in both sources keeps 'underground' so it
	-- gets the upweight.
	deduped AS (
		SELECT DISTINCT ON (track_id) track_id, source
		FROM candidates
		ORDER BY track_id, (source = 'underground') DESC
	),
	filtered AS (
		SELECT
			t.track_id,
			t.owner_id,
			t.genre,
			COALESCE(t.duration, 0) AS duration,
			d.source,
			ga.share AS genre_share,
			(fs.user_id IS NOT NULL) AS is_followed,
			(aa.artist_id IS NOT NULL) AS has_affinity,
			COALESCE(at.save_count, 0)   AS save_count,
			COALESCE(at.repost_count, 0) AS repost_count,
			COALESCE(ap.count, 0)        AS play_count
		FROM deduped d
		JOIN tracks t ON t.track_id = d.track_id
		JOIN users u  ON u.user_id = t.owner_id
		LEFT JOIN aggregate_track at ON at.track_id = t.track_id
		LEFT JOIN aggregate_plays ap ON ap.play_item_id = t.track_id
		LEFT JOIN my_genre_affinity ga ON ga.genre = t.genre
		LEFT JOIN follow_set fs ON fs.user_id = t.owner_id
		LEFT JOIN my_artist_affinity aa ON aa.artist_id = t.owner_id
		WHERE t.is_current = true
		  AND t.is_delete = false
		  AND t.is_unlisted = false
		  AND t.is_available = true
		  AND t.stem_of IS NULL
		  -- Gated tracks are excluded so the mix plays straight through.
		  AND t.is_stream_gated = false
		  AND t.created_at >= NOW() - MAKE_INTERVAL(days => @maxAgeDays::int)
		  AND t.owner_id <> @userId
		  AND u.is_current = true
		  AND u.is_deactivated = false
		  AND u.is_available = true
		  AND NOT EXISTS (SELECT 1 FROM my_played mp WHERE mp.track_id = t.track_id)
		  AND NOT EXISTS (SELECT 1 FROM my_saved ms WHERE ms.track_id = t.track_id)
		  AND t.track_id <> ALL(@excludeTrackIds::int[])
	),
	scored AS (
		SELECT
			track_id,
			owner_id,
			duration,
			LN(1 + 3 * save_count + 2 * repost_count + 1 * play_count) / 12.0
				AS quality_score,
			CASE
				WHEN genre IS NULL OR genre = ''                   THEN 1.00
				WHEN NOT EXISTS (SELECT 1 FROM my_genre_affinity)  THEN 1.00
				WHEN genre_share IS NULL                           THEN 0.85
				ELSE 0.85 + 0.45 * LEAST(genre_share / 0.30, 1.0)
			END AS genre_affinity,
			CASE
				WHEN is_followed  THEN 0.70
				WHEN has_affinity THEN 1.00
				ELSE 1.25
			END AS discovery_weight,
			CASE WHEN source = 'underground' THEN 1.15 ELSE 1.00 END
				AS source_weight,
			-- Deterministic jitter in [floor, floor + range). hashtextextended
			-- is stable across servers, so every API node computes the same mix.
			-- abs() before the mod keeps the result non-negative. @seedKey is pre-formatted
			-- because pgx infers one type per named arg and @userId is already
			-- used as an int above.
			@jitterFloor::float8 + @jitterRange::float8 * (
				(ABS(HASHTEXTEXTENDED(
					track_id::text || ':' || @seedKey::text, 0
				)) % 1000000)::float8 / 1000000.0
			) AS week_seed
		FROM filtered
	),
	final_scored AS (
		SELECT
			track_id,
			owner_id,
			duration,
			quality_score * genre_affinity * discovery_weight
				* source_weight * week_seed AS score
		FROM scored
	),
	-- One track per artist.
	capped AS (
		SELECT track_id, owner_id, duration, score,
		       ROW_NUMBER() OVER (
		           PARTITION BY owner_id ORDER BY score DESC, track_id DESC
		       ) AS rn_artist
		FROM final_scored
	)
	SELECT track_id, duration
	FROM capped
	WHERE rn_artist = 1
	-- track_id breaks score ties so the mix is byte-stable for the week.
	ORDER BY score DESC, track_id DESC
	LIMIT @limit
	`

	rows, err := app.pool.Query(ctx, sql, pgx.NamedArgs{
		"userId":          userId,
		"seedKey":         fmt.Sprintf("%d:%d:%d", userId, year, week),
		"periodStart":     weeklyrotation.PeriodStart(year, week),
		"excludeTrackIds": excludeTrackIds,
		"limit":           weeklyRotationStoredSize,
		"maxAgeDays":      weeklyRotationMaxAgeDays,
		"jitterFloor":     weeklyRotationJitterFloor,
		"jitterRange":     weeklyRotationJitterRange,
	})
	if err != nil {
		return nil, err
	}
	type rankedTrack struct {
		TrackID  int32
		Duration int32
	}
	ranked, err := pgx.CollectRows(rows, pgx.RowToStructByPos[rankedTrack])
	if err != nil {
		return nil, err
	}

	// Open with the best track that isn't a long-form set.
	for i, t := range ranked {
		if t.Duration <= weeklyRotationLongFormSeconds {
			copy(ranked[1:i+1], ranked[:i])
			ranked[0] = t
			break
		}
	}

	ids := make([]int32, len(ranked))
	for i, t := range ranked {
		ids[i] = t.TrackID
	}
	return ids, nil
}
