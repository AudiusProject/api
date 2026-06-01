package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"go.uber.org/zap"
)

// TrendingJob recomputes trending scores. Ports the score-computation half
// of apps/packages/discovery-provider/src/tasks/index_trending.py.
//
// On each run:
//  1. Refresh aggregate_interval_plays MV (feeds week/month listen counts).
//  2. Refresh trending_params MV (per-track inputs).
//  3. For each (entity_type, version), reconcile track_trending_scores /
//     playlist_trending_scores against this cycle's computed scores using an
//     upsert (skip-unchanged) plus an anti-join delete of rows that no longer
//     qualify.
//
// Cadence: discovery ran this hourly (default `trending_refresh_seconds = 3600`
// in apps' default_config.ini — its celery beat ticked every 10s but an
// internal time gate only recomputed once an hour). The vendored port dropped
// that gate, so it must be scheduled at the real cadence (see indexer's
// startParityJobs) rather than every tick.
//
// Write strategy: step 3 used to DELETE every row for a (type, version) and
// bulk-INSERT them all back on every run, rewriting the entire 16GB
// track_trending_scores table (and all six of its indexes) regardless of
// whether any score changed. Because the decay formula has day granularity,
// most per-track scores are stable between hourly runs, so we now stage scores
// in a temp table and UPSERT with a `score IS DISTINCT` guard — turning the
// common case into a near-zero-write no-op — then delete only the rows that
// dropped out of the result set.
//
// What is intentionally NOT ported here:
//   - Trending notifications (top-10 mover diff against the notification
//     table). Lands with the challenges/notifications work.
//   - index_tastemaker (writes challenge events). Depends on the challenge
//     bus, which is a separate effort.
//
// Trending score templates are copied verbatim from
// apps/packages/discovery-provider/src/trending_strategies/{pnagD,AnlGe}_*.py
// so the scoring stays bit-identical to discovery.
type TrendingJob struct {
	pool   database.DbPool
	logger *zap.Logger

	mutex     sync.Mutex
	isRunning bool
}

func NewTrendingJob(cfg config.Config, pool database.DbPool) *TrendingJob {
	return &TrendingJob{
		pool:   pool,
		logger: logging.NewZapLogger(cfg).Named("TrendingJob"),
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *TrendingJob) ScheduleEvery(ctx context.Context, interval time.Duration) *TrendingJob {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				j.Run(ctx)
			case <-ctx.Done():
				j.logger.Info("Job shutting down")
				return
			}
		}
	}()
	return j
}

// Run executes the job once.
func (j *TrendingJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

func (j *TrendingJob) run(ctx context.Context) error {
	j.mutex.Lock()
	if j.isRunning {
		j.mutex.Unlock()
		return fmt.Errorf("job is already running")
	}
	j.isRunning = true
	j.mutex.Unlock()
	defer func() {
		j.mutex.Lock()
		j.isRunning = false
		j.mutex.Unlock()
	}()

	start := time.Now()

	for _, mv := range []string{"aggregate_interval_plays", "trending_params"} {
		if err := j.refreshMatview(ctx, mv); err != nil {
			return err
		}
	}

	if err := j.computeTrendingTracks(ctx, "TRACKS", "pnagD", trackParamsPnagD); err != nil {
		return fmt.Errorf("trending tracks pnagD: %w", err)
	}
	if err := j.computeTrendingTracks(ctx, "TRACKS", "AnlGe", trackParamsAnlGe); err != nil {
		return fmt.Errorf("trending tracks AnlGe: %w", err)
	}
	// Underground uses the same TRACKS strategies but registers under a
	// different trending_type key; apps only registers pnagD for it.
	if err := j.computeTrendingTracks(ctx, "UNDERGROUND_TRACKS", "pnagD", trackParamsPnagD); err != nil {
		return fmt.Errorf("trending underground tracks pnagD: %w", err)
	}

	if err := j.computeTrendingPlaylists(ctx, "PLAYLISTS", "pnagD"); err != nil {
		return fmt.Errorf("trending playlists pnagD: %w", err)
	}

	j.logger.Info("Trending scores recomputed", zap.Duration("duration", time.Since(start)))
	return nil
}

// refreshMatview refreshes a trending matview, preferring the CONCURRENTLY
// variant so the refresh doesn't take an ACCESS EXCLUSIVE lock that stalls
// every reader (the block-indexing loop included).
//
// CONCURRENTLY has two preconditions: the matview must already be populated,
// and it must carry a unique index (added by migration 0214). Both can be
// transiently false — right after a schema-only bootstrap the matview is
// created WITH NO DATA, and during a rolling deploy the new code can land
// before the index migration. In those windows we fall back to a blocking
// refresh so the job still makes progress; the next cycle uses CONCURRENTLY
// once the preconditions hold.
func (j *TrendingJob) refreshMatview(ctx context.Context, mv string) error {
	mvStart := time.Now()
	concurrent := true
	if _, err := j.pool.Exec(ctx, "REFRESH MATERIALIZED VIEW CONCURRENTLY "+mv); err != nil {
		j.logger.Warn("concurrent matview refresh failed; falling back to blocking refresh",
			zap.String("matview", mv), zap.Error(err))
		concurrent = false
		if _, err := j.pool.Exec(ctx, "REFRESH MATERIALIZED VIEW "+mv); err != nil {
			return fmt.Errorf("refresh %s: %w", mv, err)
		}
	}
	j.logger.Info("refreshed matview",
		zap.String("matview", mv),
		zap.Bool("concurrent", concurrent),
		zap.Duration("duration", time.Since(mvStart)))
	return nil
}

// trackScoreParams holds the scalar weights used by a tracks strategy. The
// only behavior difference between pnagD and AnlGe is whether the trailing
// multiplier is `karma` or `(1 + LOG(1 + karma))`.
type trackScoreParams struct {
	karmaExpr string
}

var (
	trackParamsPnagD = trackScoreParams{karmaExpr: "tp.karma"}
	trackParamsAnlGe = trackScoreParams{karmaExpr: "(1 + LOG(1 + tp.karma))"}
)

// Score weights matching apps' constants in pnagD_trending_tracks_strategy.py
// (and AnlGe — they share the same constants).
const (
	trendingN  = 1
	trendingF  = 50
	trendingO  = 1
	trendingR  = 0.25
	trendingI  = 0.01
	trendingQ  = 100000.0
	trendingY  = 3
	trendingWk = 7
	trendingMo = 30
)

// computeTrendingTracks runs the three-time-range score insert for one
// (trending_type, version) combination. Mirrors update_track_score_query
// from apps' track strategies.
func (j *TrendingJob) computeTrendingTracks(ctx context.Context, trendingType, version string, p trackScoreParams) error {
	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Stage this cycle's scores in a temp table, then reconcile against the
	// live table (upsert changed/new rows, delete dropped-out rows). The score
	// expressions below are byte-for-byte the discovery templates; only the
	// write target changed (they now feed tmp_track_scores instead of a
	// DELETE+INSERT of track_trending_scores), and the bind parameters were
	// renumbered without gaps because the (constant) type/version no longer
	// appear in the staged SELECT — type/version are applied at the upsert.
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE tmp_track_scores (
			track_id   integer,
			genre      varchar,
			time_range varchar,
			score      double precision
		) ON COMMIT DROP
	`); err != nil {
		return fmt.Errorf("create temp: %w", err)
	}

	// Week + month follow the same shape; we generate one query per range.
	for _, r := range []struct {
		name        string
		days        int
		listenField string
		repostField string
		saveField   string
	}{
		{"week", trendingWk, "aip.week_listen_counts", "tp.repost_week_count", "tp.save_week_count"},
		{"month", trendingMo, "aip.month_listen_counts", "tp.repost_month_count", "tp.save_month_count"},
	} {
		q := fmt.Sprintf(`
			INSERT INTO tmp_track_scores (track_id, genre, time_range, score)
			SELECT
				tp.track_id,
				tp.genre,
				$1,
				CASE
				WHEN tp.owner_follower_count < $2 THEN 0
				WHEN EXTRACT(DAYS FROM now() - (
					CASE
						WHEN tp.release_date > now() THEN aip.created_at
						ELSE GREATEST(tp.release_date, aip.created_at)
					END
				)) > $3 THEN GREATEST(
					1.0 / $4,
					POW($4, GREATEST(
						-10,
						1.0 - 1.0 * EXTRACT(DAYS FROM now() - (
							CASE
								WHEN tp.release_date > now() THEN aip.created_at
								ELSE GREATEST(tp.release_date, aip.created_at)
							END
						)) / $3
					))
				) * (
					$5 * %s + $6 * %s + $7 * %s + $8 * tp.repost_count + $9 * tp.save_count
				) * %s
				ELSE (
					$5 * %s + $6 * %s + $7 * %s + $8 * tp.repost_count + $9 * tp.save_count
				) * %s
				END
			FROM trending_params tp
			INNER JOIN aggregate_interval_plays aip ON tp.track_id = aip.track_id
		`,
			r.listenField, r.repostField, r.saveField, p.karmaExpr,
			r.listenField, r.repostField, r.saveField, p.karmaExpr,
		)
		if _, err := tx.Exec(ctx, q,
			r.name,
			trendingY, r.days, trendingQ,
			trendingN, trendingF, trendingO, trendingR, trendingI,
		); err != nil {
			return fmt.Errorf("%s range: %w", r.name, err)
		}
	}

	// All-time uses aggregate_plays.count rather than aggregate_interval_plays.
	q := fmt.Sprintf(`
		INSERT INTO tmp_track_scores (track_id, genre, time_range, score)
		SELECT
			tp.track_id, tp.genre, 'allTime',
			CASE
			WHEN tp.owner_follower_count < $1 THEN 0
			ELSE ($2 * ap.count + $3 * tp.repost_count + $4 * tp.save_count) * %s
			END
		FROM trending_params tp
		INNER JOIN aggregate_plays ap ON tp.track_id = ap.play_item_id
		INNER JOIN tracks t ON ap.play_item_id = t.track_id
		WHERE t.is_current IS TRUE
		  AND t.is_delete IS FALSE
		  AND t.is_unlisted IS FALSE
		  AND t.stem_of IS NULL
	`, p.karmaExpr)
	if _, err := tx.Exec(ctx, q,
		trendingY, trendingN, trendingR, trendingI,
	); err != nil {
		return fmt.Errorf("allTime range: %w", err)
	}

	// Upsert new/changed rows; the WHERE guard skips rows whose score and genre
	// are unchanged so a no-op cycle writes (and dirties indexes for) nothing.
	if _, err := tx.Exec(ctx, `
		INSERT INTO track_trending_scores
			(track_id, genre, type, version, time_range, score, created_at)
		SELECT s.track_id, s.genre, $1, $2, s.time_range, s.score, now()
		FROM tmp_track_scores s
		ON CONFLICT (track_id, type, version, time_range) DO UPDATE
			SET score = EXCLUDED.score,
			    genre = EXCLUDED.genre,
			    created_at = EXCLUDED.created_at
			WHERE track_trending_scores.score IS DISTINCT FROM EXCLUDED.score
			   OR track_trending_scores.genre IS DISTINCT FROM EXCLUDED.genre
	`, trendingType, version); err != nil {
		return fmt.Errorf("upsert: %w", err)
	}

	// Delete rows that no longer qualify this cycle (e.g. track deleted/unlisted
	// or aged out of the interval window).
	if _, err := tx.Exec(ctx, `
		DELETE FROM track_trending_scores t
		WHERE t.type = $1 AND t.version = $2
		  AND NOT EXISTS (
		      SELECT 1 FROM tmp_track_scores s
		      WHERE s.track_id = t.track_id AND s.time_range = t.time_range
		  )
	`, trendingType, version); err != nil {
		return fmt.Errorf("prune stale: %w", err)
	}

	return tx.Commit(ctx)
}

// computeTrendingPlaylists recomputes playlist_trending_scores for the given
// (type, version). Ports apps' pnagD playlist score template.
func (j *TrendingJob) computeTrendingPlaylists(ctx context.Context, trendingType, version string) error {
	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Stage this cycle's scores, then upsert/prune (see computeTrendingTracks
	// for the rationale). The per-range SELECT is the verbatim discovery
	// template; only its write target changed, and the bind parameters were
	// renumbered without gaps now that the (constant) type/version are applied
	// at the upsert instead of in the staged SELECT.
	if _, err := tx.Exec(ctx, `
		CREATE TEMP TABLE tmp_playlist_scores (
			playlist_id integer,
			time_range  varchar,
			score       double precision
		) ON COMMIT DROP
	`); err != nil {
		return fmt.Errorf("create temp: %w", err)
	}

	// Each time range gets its own query because the interval (':week days'
	// vs ':month days' vs ':year days') is part of the WHERE clauses.
	for _, r := range []struct {
		name string
		days int
	}{
		{"week", trendingWk},
		{"month", trendingMo},
		{"year", 365},
	} {
		intervalLit := fmt.Sprintf("%d days", r.days)
		q := `
			INSERT INTO tmp_playlist_scores (playlist_id, time_range, score)
			WITH saves_and_reposts AS (
			    SELECT user_id, repost_item_id AS item_id
			    FROM reposts
			    WHERE is_current IS TRUE
			      AND is_delete IS FALSE
			      AND repost_type = 'playlist'
			      AND repost_item_id IN (SELECT playlist_id FROM playlists WHERE is_current IS TRUE AND is_delete IS FALSE AND is_private IS FALSE)
			      AND created_at >= NOW() - $1::interval
			    UNION ALL
			    SELECT user_id, save_item_id AS item_id
			    FROM saves
			    WHERE is_current IS TRUE
			      AND is_delete IS FALSE
			      AND save_type = 'playlist'
			      AND save_item_id IN (SELECT playlist_id FROM playlists WHERE is_current IS TRUE AND is_delete IS FALSE AND is_private IS FALSE)
			      AND created_at >= NOW() - $1::interval
			),
			filtered_users AS (
			    SELECT sr.user_id, sr.item_id
			    FROM saves_and_reposts sr
			    JOIN users u ON sr.user_id = u.user_id
			    WHERE (u.cover_photo IS NOT NULL OR u.cover_photo_sizes IS NOT NULL)
			      AND (u.profile_picture IS NOT NULL OR u.profile_picture_sizes IS NOT NULL)
			      AND u.bio IS NOT NULL
			      AND u.is_current IS TRUE
			),
			karma_scores AS (
			    SELECT item_id, CAST(SUM(au.follower_count) AS integer) AS karma
			    FROM filtered_users f
			    JOIN aggregate_user au ON f.user_id = au.user_id
			    GROUP BY item_id
			),
			windowed_saves AS (
			    SELECT save_item_id, COUNT(*) AS week_count
			    FROM saves
			    WHERE is_current IS TRUE
			      AND is_delete IS FALSE
			      AND save_type = 'playlist'
			      AND created_at > NOW() - $1::interval
			    GROUP BY save_item_id
			),
			windowed_reposts AS (
			    SELECT repost_item_id, COUNT(*) AS week_count
			    FROM reposts
			    WHERE is_current IS TRUE
			      AND is_delete IS FALSE
			      AND repost_type = 'playlist'
			      AND created_at > NOW() - $1::interval
			    GROUP BY repost_item_id
			)
			SELECT
			    p.playlist_id,
			    $2,
			    CASE
			    WHEN au.follower_count < $3 THEN 0
			    WHEN EXTRACT(DAYS FROM now() - (
			        CASE WHEN p.release_date > now() THEN p.created_at
			             ELSE GREATEST(p.release_date, p.created_at) END
			    )) > $4 THEN GREATEST(
			        1.0 / $5,
			        POW($5, GREATEST(
			            -10,
			            1.0 - 1.0 * EXTRACT(DAYS FROM now() - (
			                CASE WHEN p.release_date > now() THEN p.created_at
			                     ELSE GREATEST(p.release_date, p.created_at) END
			            )) / $4
			        ))
			    ) * (
			        $6 * 1 + $7 * COALESCE(rp.week_count, 0) + $8 * COALESCE(s.week_count, 0)
			          + $9 * COALESCE(ap.repost_count, 0) + $10 * COALESCE(ap.save_count, 0)
			    ) * COALESCE(k.karma, 1)
			    ELSE (
			        $6 * 1 + $7 * COALESCE(rp.week_count, 0) + $8 * COALESCE(s.week_count, 0)
			          + $9 * COALESCE(ap.repost_count, 0) + $10 * COALESCE(ap.save_count, 0)
			    ) * COALESCE(k.karma, 1)
			    END
			FROM playlists p
			INNER JOIN aggregate_user au ON p.playlist_owner_id = au.user_id
			LEFT JOIN aggregate_playlist ap ON p.playlist_id = ap.playlist_id
			LEFT JOIN windowed_saves s ON p.playlist_id = s.save_item_id
			LEFT JOIN windowed_reposts rp ON p.playlist_id = rp.repost_item_id
			LEFT JOIN karma_scores k ON p.playlist_id = k.item_id
			WHERE p.is_current IS TRUE
			  AND p.is_delete IS FALSE
			  AND p.is_private IS FALSE
			  AND jsonb_array_length(p.playlist_contents->'track_ids') >= 3
		`
		if _, err := tx.Exec(ctx, q,
			intervalLit, r.name,
			trendingY, r.days, trendingQ,
			trendingN, trendingF, trendingO, trendingR, trendingI,
		); err != nil {
			return fmt.Errorf("%s range: %w", r.name, err)
		}
	}

	// Upsert new/changed rows; skip rows whose score is unchanged.
	if _, err := tx.Exec(ctx, `
		INSERT INTO playlist_trending_scores
			(playlist_id, type, version, time_range, score, created_at)
		SELECT s.playlist_id, $1, $2, s.time_range, s.score, now()
		FROM tmp_playlist_scores s
		ON CONFLICT (playlist_id, type, version, time_range) DO UPDATE
			SET score = EXCLUDED.score,
			    created_at = EXCLUDED.created_at
			WHERE playlist_trending_scores.score IS DISTINCT FROM EXCLUDED.score
	`, trendingType, version); err != nil {
		return fmt.Errorf("upsert: %w", err)
	}

	// Delete rows that no longer qualify this cycle.
	if _, err := tx.Exec(ctx, `
		DELETE FROM playlist_trending_scores t
		WHERE t.type = $1 AND t.version = $2
		  AND NOT EXISTS (
		      SELECT 1 FROM tmp_playlist_scores s
		      WHERE s.playlist_id = t.playlist_id AND s.time_range = t.time_range
		  )
	`, trendingType, version); err != nil {
		return fmt.Errorf("prune stale: %w", err)
	}

	return tx.Commit(ctx)
}
