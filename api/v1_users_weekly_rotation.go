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
)

/*
Returns a taste-matched track mix that is stable for the rotation period (the
"Weekly Rotation" surface). The period rolls over on Wednesday 00:00 UTC, see
weeklyrotation.RolloverOffsetDays.

Differences from GET /v1/users/{id}/feed/for-you:
  - The mix is fixed for the period instead of re-ranked on every load.
  - Followed artists are demoted instead of boosted.
  - Played and saved tracks are excluded instead of soft-penalized.

STABILITY. Nothing is precomputed or stored. The query is deterministic given
(user_id, iso_year, iso_week) and the result is cached until the period rolls.
The week-to-week variation comes from week_seed, a hash of (track_id, user_id,
year, week); nothing uses random().

The listener's history (plays, saves, reposts, follows) is read as of the
period start, so listening to the mix doesn't change it mid-week. The
candidate pool and engagement counts are still live, so comparable tracks can
reorder during the week. Unsaves and unfollows during the period can still add
tracks.

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
anything played, anything saved, and anything older than
weeklyRotationMaxAgeDays.

DIVERSITY. One track per artist (For You allows 3).

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
	cacheKey := fmt.Sprintf("weekly_rotation:%d:%d:%d:%d", userId, year, week, limit)
	if hit, ok := app.weeklyRotationCache.Get(cacheKey); ok {
		return hit, nil
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
	),
	scored AS (
		SELECT
			track_id,
			owner_id,
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
			quality_score * genre_affinity * discovery_weight
				* source_weight * week_seed AS score
		FROM scored
	),
	-- One track per artist.
	capped AS (
		SELECT track_id, owner_id, score,
		       ROW_NUMBER() OVER (
		           PARTITION BY owner_id ORDER BY score DESC, track_id DESC
		       ) AS rn_artist
		FROM final_scored
	)
	SELECT track_id
	FROM capped
	WHERE rn_artist = 1
	-- track_id breaks score ties so the mix is byte-stable for the week.
	ORDER BY score DESC, track_id DESC
	LIMIT @limit
	`

	rows, err := app.pool.Query(ctx, sql, pgx.NamedArgs{
		"userId":      userId,
		"seedKey":     fmt.Sprintf("%d:%d:%d", userId, year, week),
		"periodStart": weeklyrotation.PeriodStart(year, week),
		"limit":       limit,
		"maxAgeDays":  weeklyRotationMaxAgeDays,
		"jitterFloor": weeklyRotationJitterFloor,
		"jitterRange": weeklyRotationJitterRange,
	})
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return nil, err
	}

	app.weeklyRotationCache.Set(cacheKey, ids)
	return ids, nil
}
