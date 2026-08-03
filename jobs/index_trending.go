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
// On each run it refreshes the aggregate_interval_plays and trending_params
// matviews (CONCURRENTLY, so readers aren't blocked — see refreshMatview), then
// for each (entity_type, version) clears that slice of the *_trending_scores
// table and repopulates it with plain bulk INSERTs, all in one transaction so
// readers never see an empty table.
//
// This DELETE-all + bulk-INSERT is what discovery's Python did for years. A
// previous attempt to replace it with a skip-unchanged upsert (temp-table stage
// + INSERT ... ON CONFLICT DO UPDATE + anti-join prune) aimed to avoid rewriting
// the whole multi-GB table and its indexes each run, but in practice the per-row
// unique-index probe over ~4M rows took 40+ minutes and never finished inside
// the hourly window, so scores went stale. The bulk rewrite avoids that probe
// storm, and this job only persists positive track scores to keep the rewrite
// and index maintenance bounded as the track corpus grows.
//
// Scheduled hourly to match discovery (default trending_refresh_seconds=3600;
// see indexer's startParityJobs). Score expressions are copied from apps'
// trending_strategies/pnagD_*.py so scoring stays bit-identical.
type TrendingJob struct {
	pool   database.DbPool
	logger *zap.Logger

	mutex     sync.Mutex
	isRunning bool
	now       func() time.Time
}

func NewTrendingJob(cfg config.Config, pool database.DbPool) *TrendingJob {
	return &TrendingJob{
		pool:   pool,
		logger: logging.NewZapLogger(cfg).Named("TrendingJob"),
		now:    time.Now,
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
// The interval is measured after each completed run, so a slow run does not
// leave a pending ticker event that immediately starts another DB-heavy pass.
func (j *TrendingJob) ScheduleEvery(ctx context.Context, interval time.Duration) *TrendingJob {
	go func() {
		timer := time.NewTimer(interval)
		defer timer.Stop()
		for {
			select {
			case <-timer.C:
				j.Run(ctx)
				timer.Reset(interval)
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

	if err := j.computeTrendingPlaylists(ctx, "PLAYLISTS", "pnagD"); err != nil {
		return fmt.Errorf("trending playlists pnagD: %w", err)
	}

	if err := j.computeTrendingTrackAllTimeIfDue(ctx, "TRACKS", "pnagD", trackParamsPnagD); err != nil {
		return fmt.Errorf("trending tracks pnagD allTime: %w", err)
	}

	j.logger.Info("Trending scores recomputed", zap.Duration("duration", time.Since(start)))
	return nil
}

// refreshMatview prefers REFRESH ... CONCURRENTLY so it doesn't take an ACCESS
// EXCLUSIVE lock that stalls every reader (the block-indexing loop included).
// CONCURRENTLY needs the matview populated and carrying a unique index (migration
// 0214); both can be transiently false (fresh schema-only DB, or pre-migration
// during a rolling deploy), so we fall back to a blocking refresh until they hold.
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

// trackScoreParams holds the scalar weights used by a tracks strategy.
type trackScoreParams struct {
	karmaExpr string
}

var trackParamsPnagD = trackScoreParams{karmaExpr: "tp.karma"}

// Score weights matching apps' constants in pnagD_trending_tracks_strategy.py
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

	trendingAllTimeCheckpoint = "track_trending_scores_all_time"
	trendingAllTimeInterval   = 24 * time.Hour
)

// computeTrendingTracks runs the week/month score inserts for one
// (trending_type, version) combination. Mirrors update_track_score_query
// from apps' track strategies.
func (j *TrendingJob) computeTrendingTracks(ctx context.Context, trendingType, version string, p trackScoreParams) error {
	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Mirror apps' update_track_score_query shape: clear this (type, version)'s
	// rolling rows and repopulate with plain bulk INSERTs, all inside the one
	// transaction opened above so readers never observe missing rolling rows.
	//
	// An earlier temp-table + ON CONFLICT DO UPDATE reconcile turned this into a
	// ~4M-row-per-cycle upsert — one unique-index probe and IS DISTINCT FROM
	// comparison per row — which took 40+ minutes and could never finish inside
	// the refresh interval. The blunt DELETE + append matches the proven Python
	// behavior. allTime is intentionally refreshed separately and less often.
	// The score expressions below are intentionally kept aligned with the
	// discovery templates, then filtered so we only persist tracks that can
	// actually appear ahead of the zero-score tail.
	if _, err := tx.Exec(ctx,
		`DELETE FROM track_trending_scores
		 WHERE type = $1 AND version = $2 AND time_range IN ('week', 'month')`,
		trendingType, version,
	); err != nil {
		return fmt.Errorf("delete existing: %w", err)
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
			INSERT INTO track_trending_scores
				(track_id, genre, type, version, time_range, score, created_at)
			SELECT
				track_id,
				genre,
				trending_type,
				trending_version,
				score_time_range,
				score,
				created_at
			FROM (
				SELECT
					tp.track_id,
					tp.genre,
					$1::varchar AS trending_type,
					$2::varchar AS trending_version,
					$3::varchar AS score_time_range,
					CASE
					WHEN tp.owner_follower_count < $4 THEN 0
					WHEN EXTRACT(DAYS FROM now() - (
						CASE
							WHEN tp.release_date > now() THEN aip.created_at
							ELSE GREATEST(tp.release_date, aip.created_at)
						END
					)) > $5 THEN GREATEST(
						1.0 / $6,
						POW($6, GREATEST(
							-10,
							1.0 - 1.0 * EXTRACT(DAYS FROM now() - (
								CASE
									WHEN tp.release_date > now() THEN aip.created_at
									ELSE GREATEST(tp.release_date, aip.created_at)
								END
							)) / $5
						))
					) * (
						$7 * %s + $8 * %s + $9 * %s + $10 * tp.repost_count + $11 * tp.save_count
					) * %s
					ELSE (
						$7 * %s + $8 * %s + $9 * %s + $10 * tp.repost_count + $11 * tp.save_count
					) * %s
					END AS score,
					now() AS created_at
				FROM trending_params tp
				INNER JOIN aggregate_interval_plays aip ON tp.track_id = aip.track_id
			) scored
			WHERE score > 0
		`,
			r.listenField, r.repostField, r.saveField, p.karmaExpr,
			r.listenField, r.repostField, r.saveField, p.karmaExpr,
		)
		if _, err := tx.Exec(ctx, q,
			trendingType, version,
			r.name,
			trendingY, r.days, trendingQ,
			trendingN, trendingF, trendingO, trendingR, trendingI,
		); err != nil {
			return fmt.Errorf("%s range: %w", r.name, err)
		}
	}

	return tx.Commit(ctx)
}

func (j *TrendingJob) computeTrendingTrackAllTimeIfDue(ctx context.Context, trendingType, version string, p trackScoreParams) error {
	now := j.now().UTC()
	lastRunUnix, err := getCheckpoint(ctx, j.pool, trendingAllTimeCheckpoint)
	if err != nil {
		return fmt.Errorf("read allTime checkpoint: %w", err)
	}
	if lastRunUnix > 0 {
		lastRun := time.Unix(lastRunUnix, 0).UTC()
		if now.Sub(lastRun) < trendingAllTimeInterval {
			j.logger.Debug("Skipping allTime trending refresh",
				zap.Time("last_run", lastRun),
				zap.Duration("next_run_in", trendingAllTimeInterval-now.Sub(lastRun)))
			return nil
		}
	}

	return j.computeTrendingTrackAllTime(ctx, trendingType, version, p, now)
}

// computeTrendingTrackAllTime refreshes only the allTime range. It is kept out
// of the hourly week/month pass because it scans aggregate_plays and changes
// slowly compared with rolling windows.
func (j *TrendingJob) computeTrendingTrackAllTime(ctx context.Context, trendingType, version string, p trackScoreParams, checkpointTime time.Time) error {
	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx,
		`DELETE FROM track_trending_scores
		 WHERE type = $1 AND version = $2 AND time_range = 'allTime'`,
		trendingType, version,
	); err != nil {
		return fmt.Errorf("delete existing allTime: %w", err)
	}

	q := fmt.Sprintf(`
		INSERT INTO track_trending_scores
			(track_id, genre, type, version, time_range, score, created_at)
		SELECT
			track_id,
			genre,
			trending_type,
			trending_version,
			time_range,
			score,
			created_at
		FROM (
			SELECT
				tp.track_id,
				tp.genre,
				$1::varchar AS trending_type,
				$2::varchar AS trending_version,
				'allTime'::varchar AS time_range,
				CASE
				WHEN tp.owner_follower_count < $3 THEN 0
				ELSE ($4 * ap.count + $5 * tp.repost_count + $6 * tp.save_count) * %s
				END AS score,
				now() AS created_at
			FROM trending_params tp
			INNER JOIN aggregate_plays ap ON tp.track_id = ap.play_item_id
			INNER JOIN tracks t ON ap.play_item_id = t.track_id
			WHERE t.is_current IS TRUE
			  AND t.is_delete IS FALSE
			  AND t.is_unlisted IS FALSE
			  AND t.stem_of IS NULL
		) scored
		WHERE score > 0
	`, p.karmaExpr)
	if _, err := tx.Exec(ctx, q,
		trendingType, version,
		trendingY, trendingN, trendingR, trendingI,
	); err != nil {
		return fmt.Errorf("allTime range: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		INSERT INTO indexing_checkpoints (tablename, last_checkpoint)
		VALUES ($1, $2)
		ON CONFLICT (tablename) DO UPDATE SET last_checkpoint = EXCLUDED.last_checkpoint
	`, trendingAllTimeCheckpoint, checkpointTime.Unix()); err != nil {
		return fmt.Errorf("save allTime checkpoint: %w", err)
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

	// Clear this (type, version)'s rows and repopulate with plain bulk INSERTs in
	// the single transaction opened above — same DELETE+INSERT strategy as
	// computeTrendingTracks (see the rationale there). The per-range SELECT is
	// the verbatim discovery template.
	if _, err := tx.Exec(ctx,
		`DELETE FROM playlist_trending_scores WHERE type = $1 AND version = $2`,
		trendingType, version,
	); err != nil {
		return fmt.Errorf("delete existing: %w", err)
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
			INSERT INTO playlist_trending_scores
				(playlist_id, type, version, time_range, score, created_at)
			WITH saves_and_reposts AS (
			    SELECT user_id, repost_item_id AS item_id
			    FROM reposts
			    WHERE is_current IS TRUE
			      AND is_delete IS FALSE
			      AND repost_type = 'playlist'
			      AND repost_item_id IN (SELECT playlist_id FROM playlists WHERE is_current IS TRUE AND is_delete IS FALSE AND is_private IS FALSE)
			      AND created_at >= NOW() - $3::interval
			    UNION ALL
			    SELECT user_id, save_item_id AS item_id
			    FROM saves
			    WHERE is_current IS TRUE
			      AND is_delete IS FALSE
			      AND save_type = 'playlist'
			      AND save_item_id IN (SELECT playlist_id FROM playlists WHERE is_current IS TRUE AND is_delete IS FALSE AND is_private IS FALSE)
			      AND created_at >= NOW() - $3::interval
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
			      AND created_at > NOW() - $3::interval
			    GROUP BY save_item_id
			),
			windowed_reposts AS (
			    SELECT repost_item_id, COUNT(*) AS week_count
			    FROM reposts
			    WHERE is_current IS TRUE
			      AND is_delete IS FALSE
			      AND repost_type = 'playlist'
			      AND created_at > NOW() - $3::interval
			    GROUP BY repost_item_id
			)
			SELECT
			    p.playlist_id,
			    $1,
			    $2,
			    $4,
			    CASE
			    WHEN au.follower_count < $5 THEN 0
			    WHEN EXTRACT(DAYS FROM now() - (
			        CASE WHEN p.release_date > now() THEN p.created_at
			             ELSE GREATEST(p.release_date, p.created_at) END
			    )) > $6 THEN GREATEST(
			        1.0 / $7,
			        POW($7, GREATEST(
			            -10,
			            1.0 - 1.0 * EXTRACT(DAYS FROM now() - (
			                CASE WHEN p.release_date > now() THEN p.created_at
			                     ELSE GREATEST(p.release_date, p.created_at) END
			            )) / $6
			        ))
			    ) * (
			        $8 * 1 + $9 * COALESCE(rp.week_count, 0) + $10 * COALESCE(s.week_count, 0)
			          + $11 * COALESCE(ap.repost_count, 0) + $12 * COALESCE(ap.save_count, 0)
			    ) * COALESCE(k.karma, 1)
			    ELSE (
			        $8 * 1 + $9 * COALESCE(rp.week_count, 0) + $10 * COALESCE(s.week_count, 0)
			          + $11 * COALESCE(ap.repost_count, 0) + $12 * COALESCE(ap.save_count, 0)
			    ) * COALESCE(k.karma, 1)
			    END,
			    now()
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
			trendingType, version,
			intervalLit, r.name,
			trendingY, r.days, trendingQ,
			trendingN, trendingF, trendingO, trendingR, trendingI,
		); err != nil {
			return fmt.Errorf("%s range: %w", r.name, err)
		}
	}

	return tx.Commit(ctx)
}
