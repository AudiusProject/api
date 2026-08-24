package api

import (
	"context"
	"fmt"
	"time"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersDiscoverWeeklyParams struct {
	Limit int `query:"limit" default:"30" validate:"min=1,max=50"`
}

const (
	// Tracks older than this are excluded outright. A discovery mix is
	// allowed to reach much further back than the For You feed (48h
	// half-life), but a track from 2019 that never found an audience is
	// usually not a hidden gem — it's an abandoned upload.
	discoverWeeklyMaxAgeDays = 365

	// Week-seeded jitter band. Scores across the candidate pool are tightly
	// clustered, so without a deterministic per-week perturbation the same
	// user would get a near-identical mix every week. +/-15% is enough to
	// rotate the ordering among comparable candidates without letting a weak
	// track outrank a genuinely better one.
	discoverWeeklyJitterFloor = 0.85
	discoverWeeklyJitterRange = 0.30
)

/*
Returns a fixed-size, taste-matched track mix that is stable for the
calendar week — the "Discover Weekly" surface.

Distinct from GET /v1/users/{id}/feed/for-you in three ways that matter:

  - For You is a *feed*: freshness-weighted (48h half-life), re-ranked on
    every load, infinite. This is an *artifact*: a fixed 30 tracks that do
    not change until the week rolls over, so it can be linked, revisited,
    and talked about.
  - For You boosts in-network (followed) creators. This demotes them. The
    point of the mix is artists the listener hasn't found yet, so a
    followed artist has to clear a higher bar to appear.
  - For You soft-penalizes tracks you've already heard. This excludes them
    outright, along with anything you've saved. A mix with a track you
    already know in it reads as broken.

STABILITY. There is no precompute job and no stored playlist. Audius
playlists are on-chain entities, so minting one per user per week is not
on the table; instead the query is fully deterministic given
(user_id, iso_year, iso_week) and the result is cached until the week
rolls. Nothing here uses random() — the week-to-week variation comes from
`week_seed` below, which is a hash of (track_id, user_id, year, week).
Same inputs, same mix, all week.

SCORING.

	quality_score  = ln(1 + 3*saves + 2*reposts + 1*plays) / 12
	                 // same log-compressed engagement blend as For You:
	                 // saves > reposts > plays.
	genre_affinity = 0.85 + 0.45 * min(genre_share / 0.30, 1)
	                 // genre_share is the fraction of my recent plays in
	                 // the track's genre. Carried over from For You
	                 // unchanged — this is the taste signal.
	discovery_wt   = {not followed, low affinity: 1.25,
	                  not followed, some affinity: 1.00,
	                  followed:                    0.70}
	                 // inverted relative to For You's in-network boost.
	source_weight  = {underground: 1.15, trending: 1.00}
	                 // underground is upweighted here; the whole surface
	                 // exists to promote things the listener wouldn't have
	                 // stumbled into on the trending page.
	week_seed      = 0.85 + 0.30 * hash01(track_id, user_id, year, week)

	final_score = quality_score * genre_affinity * discovery_wt
	              * source_weight * week_seed

FILTERS. Track liveness (is_delete / is_unlisted / is_available /
stem_of), owner liveness (is_deactivated / is_available), gated tracks
excluded entirely (a mix the listener can't play through is worse than a
shorter mix), own uploads, anything played, anything saved, and anything
older than discoverWeeklyMaxAgeDays.

DIVERSITY. One track per artist, hard. For You allows 3 because a feed is
expected to show you more from someone you follow; a 30-track mix with two
tracks from the same artist has wasted a slot.

Path:
  - id (required): the user being personalized for. Resolved by
    requireUserIdMiddleware.

Query params:
  - limit (default 30, max 50)
  - user_id (optional): the caller, for viewer-relative fields on the
    returned tracks. Independent of the path id, same as elsewhere.
*/
func (app *ApiServer) v1UsersDiscoverWeekly(c *fiber.Ctx) error {
	params := GetUsersDiscoverWeeklyParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	userId := app.getUserId(c)
	myId := app.getMyId(c)

	year, week := discoverWeeklyPeriod(time.Now().UTC())

	trackIds, err := app.getDiscoverWeeklyTrackIds(
		c.Context(),
		userId,
		year,
		week,
		params.Limit,
	)
	if err != nil {
		return err
	}

	// Tracks returns in the order of the id list, which is the product here.
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

// discoverWeeklyPeriod returns the ISO year and ISO week that `t` falls in.
// The mix is keyed on this pair, so it changes exactly once a week at the
// ISO week boundary (Monday 00:00 UTC).
func discoverWeeklyPeriod(t time.Time) (int, int) {
	return t.ISOWeek()
}

func (app *ApiServer) getDiscoverWeeklyTrackIds(
	ctx context.Context,
	userId int32,
	year int,
	week int,
	limit int,
) ([]int32, error) {
	cacheKey := fmt.Sprintf("discover_weekly:%d:%d:%d:%d", userId, year, week, limit)
	if hit, ok := app.discoverWeeklyCache.Get(cacheKey); ok {
		return hit, nil
	}

	sql := `
	WITH
	-- Everything the listener has already heard. Unlike the For You feed,
	-- which soft-penalizes repeats, these are excluded outright, so the
	-- window is wider than that endpoint's 14 days. Still bounded: a heavy
	-- listener has hundreds of thousands of play rows and an unbounded scan
	-- is what put the older recommendation endpoints over the upstream
	-- timeout (see PRs #805, #806).
	my_played AS (
		SELECT DISTINCT play_item_id AS track_id
		FROM (
			SELECT play_item_id
			FROM plays
			WHERE user_id = @userId
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
	),
	-- Capped the same way as the For You feed's follow_set, and for the
	-- same reason: a power user with thousands of follows otherwise pulls a
	-- hash table wide enough to stall the planner on the join below.
	follow_set AS (
		SELECT followee_user_id AS user_id
		FROM follows
		WHERE follower_user_id = @userId
		  AND is_current = true
		  AND is_delete = false
		ORDER BY created_at DESC
		LIMIT 500
	),
	-- Genre mix of recent listening. Identical to the For You feed's
	-- my_genre_affinity — this is the part of the taste model the two
	-- surfaces genuinely share.
	my_genre_affinity AS (
		SELECT t.genre,
		       COUNT(*)::double precision / SUM(COUNT(*)) OVER () AS share
		FROM (
			SELECT play_item_id AS track_id
			FROM plays
			WHERE user_id = @userId
			ORDER BY created_at DESC
			LIMIT 1000
		) p
		JOIN tracks t ON t.track_id = p.track_id
		WHERE t.genre IS NOT NULL AND t.genre <> ''
		GROUP BY t.genre
	),
	-- Owners the listener already engages with. Used to demote, not boost:
	-- an artist whose tracks they already save is by definition not a
	-- discovery. Bounded by recency like the For You affinity CTE.
	my_artist_affinity AS (
		SELECT owner_id AS artist_id
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
		) eng
		GROUP BY owner_id
	),
	-- Source 1: weekly trending tracks.
	--
	-- No genre predicate, deliberately. track_trending_scores holds two
	-- populations: rows carrying a genre, which are the live list the trending
	-- job refreshes (median track age ~4 days), and rows with a null/empty
	-- genre, which are stale -- in production those resolve to tracks five to
	-- six years old. GET /tracks/trending and /tracks/trending/underground read
	-- the live rows by omitting the genre filter, so this does the same.
	-- Matching on a null-or-empty genre instead reads the stale population and,
	-- combined with the age cutoff below, returns nothing at all.
	cand_trending AS (
		SELECT tts.track_id, 'trending'::text AS source
		FROM track_trending_scores tts
		WHERE tts.type = 'TRACKS'
		  AND tts.version = 'pnagD'
		  AND tts.time_range = 'week'
		ORDER BY tts.score DESC, tts.track_id DESC
		LIMIT 400
	),
	-- Source 2: the same trending slice restricted to small creators. The
	-- mirror of GET /tracks/trending/underground, and upweighted below.
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
	-- One row per track. Underground sorts before trending so a track that
	-- qualifies for both keeps the upweighted source.
	deduped AS (
		SELECT DISTINCT ON (track_id) track_id, source
		FROM candidates
		ORDER BY track_id, source ASC
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
		  -- Gated tracks are dropped rather than surfaced-and-locked: a mix
		  -- the listener can't play straight through is worse than a short one.
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
			-- hashtextextended is stable across sessions and servers, unlike
			-- hashtext's platform-dependent variants, so every API node
			-- computes the same mix for the same week. abs() then a mod into
			-- [0,1): a plain (x % n) can be negative for negative x.
			--
			-- The listener/period half of the seed arrives pre-formatted as
			-- @seedKey rather than as separate int params: pgx infers one type
			-- per named arg, and @userId is already pinned to int by the
			-- equality predicates above, so casting it to text here would
			-- conflict.
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
	-- One track per artist, hard.
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
		"limit":       limit,
		"maxAgeDays":  discoverWeeklyMaxAgeDays,
		"jitterFloor": discoverWeeklyJitterFloor,
		"jitterRange": discoverWeeklyJitterRange,
	})
	if err != nil {
		return nil, err
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return nil, err
	}

	app.discoverWeeklyCache.Set(cacheKey, ids)
	return ids, nil
}
