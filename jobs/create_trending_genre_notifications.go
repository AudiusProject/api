package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// TrendingGenreNotificationsJob tells artists when a track is in the top
// trendingGenreTopN of its genre's weekly chart, e.g. "[track] is #4 in
// Dubstep this week". One trending_genre notification per track per week;
// pedalboard turns the row into a push, gated by the push_trending_genre
// remote-config variable.
//
// The chart is the live week ranking in track_trending_scores (what
// /tracks/trending?genre= serves), snapshotted once a week on Friday, the same
// day trending winners are picked. Only genres with at least
// trendingGenreMinChartSize tracks on the chart count, so a thin genre doesn't
// produce "#4 out of 6". Tracks that already got an overall trending
// notification that day are skipped.
//
// Sends start at trendingGenreSendHourUTC on Friday and last
// trendingGenreSendWindow. Stage sends any day, keyed by the latest Friday.
//
// Scores are recomputed hourly, so the snapshot is taken once: the insert runs
// under an advisory lock and stops if any row for the week exists.
// uq_notification (group_id, specifier) covers reruns and multiple replicas.
type TrendingGenreNotificationsJob struct {
	pool   database.DbPool
	logger *zap.Logger
	env    string
	now    func() time.Time

	mutex     sync.Mutex
	isRunning bool
	doneWeek  string
}

const (
	trendingGenreSendHourUTC  = 18
	trendingGenreSendWindow   = 24 * time.Hour
	trendingGenreTopN         = 10
	trendingGenreMinChartSize = 10
	trendingGenreVersion      = "pnagD"
	trendingGenreLockKey      = 7_349_201
)

func NewTrendingGenreNotificationsJob(cfg config.Config, pool database.DbPool) *TrendingGenreNotificationsJob {
	return &TrendingGenreNotificationsJob{
		pool:   pool,
		logger: logging.NewZapLogger(cfg).Named("TrendingGenreNotificationsJob"),
		env:    cfg.Env,
		now:    time.Now,
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *TrendingGenreNotificationsJob) ScheduleEvery(ctx context.Context, interval time.Duration) *TrendingGenreNotificationsJob {
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
func (j *TrendingGenreNotificationsJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

// latestFriday returns 00:00 UTC of the most recent Friday on or before now.
func latestFriday(now time.Time) time.Time {
	now = now.UTC()
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	back := (int(day.Weekday()) - int(time.Friday) + 7) % 7
	return day.AddDate(0, 0, -back)
}

// sendWindow returns the week's Friday and whether now is inside its window.
func (j *TrendingGenreNotificationsJob) sendWindow(now time.Time) (time.Time, bool) {
	friday := latestFriday(now)
	if j.env == "stage" {
		return friday, true
	}
	start := friday.Add(trendingGenreSendHourUTC * time.Hour)
	return friday, !now.Before(start) && now.Before(start.Add(trendingGenreSendWindow))
}

func (j *TrendingGenreNotificationsJob) run(ctx context.Context) error {
	start := time.Now()
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

	now := j.now()
	friday, ok := j.sendWindow(now)
	if !ok {
		return nil
	}
	week := friday.Format("2006-01-02")
	if j.doneWeek == week {
		return nil
	}

	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, trendingGenreLockKey); err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	var sent bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM notification
			WHERE type = 'trending_genre' AND group_id LIKE @prefix || '%'
		)`, pgx.NamedArgs{"prefix": "trending_genre:" + week + ":"}).Scan(&sent); err != nil {
		return fmt.Errorf("check sent: %w", err)
	}
	if sent {
		j.doneWeek = week
		return nil
	}

	tag, err := tx.Exec(ctx, `
		WITH chart AS (
			SELECT
				s.track_id,
				s.genre,
				t.owner_id,
				ROW_NUMBER() OVER (PARTITION BY s.genre ORDER BY s.score DESC, s.track_id DESC) AS rank,
				COUNT(*) OVER (PARTITION BY s.genre) AS chart_size
			FROM track_trending_scores s
			JOIN tracks t ON t.track_id = s.track_id
				AND t.is_current
				AND NOT t.is_delete
				AND NOT t.is_unlisted
				AND t.is_available
			JOIN users u ON u.user_id = t.owner_id
				AND u.is_current
				AND NOT u.is_deactivated
				AND u.is_available
			WHERE s.type = 'TRACKS'
				AND s.version = @version
				AND s.time_range = 'week'
				AND s.genre IS NOT NULL AND s.genre <> ''
		)
		INSERT INTO notification (specifier, group_id, blocknumber, user_ids, type, data, timestamp)
		SELECT
			c.track_id::text,
			@prefix || c.track_id::text,
			NULL,
			ARRAY[c.owner_id],
			'trending_genre',
			jsonb_build_object(
				'track_id', c.track_id,
				'genre', c.genre,
				'rank', c.rank,
				'week', @week::text
			),
			@now
		FROM chart c
		WHERE c.rank <= @top_n
			AND c.chart_size >= @min_size
			AND NOT EXISTS (
				SELECT 1 FROM notification n
				WHERE n.type = 'trending'
					AND n.specifier = c.track_id::text
					AND n.timestamp >= @friday
			)
		ON CONFLICT (group_id, specifier) DO NOTHING
	`, pgx.NamedArgs{
		"version":  trendingGenreVersion,
		"prefix":   "trending_genre:" + week + ":",
		"week":     week,
		"friday":   friday,
		"now":      now,
		"top_n":    trendingGenreTopN,
		"min_size": trendingGenreMinChartSize,
	})
	if err != nil {
		return fmt.Errorf("insert trending_genre notifications: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return err
	}
	j.doneWeek = week

	j.logger.Info("Inserted trending_genre notifications",
		zap.String("week", week),
		zap.Int64("count", tag.RowsAffected()),
		zap.Duration("duration", time.Since(start)))
	return nil
}
