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

// ListenStreakReminderJob reminds users whose listen streak is about to lapse.
// Mirrors
// apps/packages/discovery-provider/src/tasks/create_listen_streak_reminder_notifications.py.
//
// A streak is kept alive by listening at least once every 48 hours. We notify
// in the hour-wide window 42-43 hours after the last listen — 6 hours of slack
// before the streak breaks. The (group_id, specifier) pair encodes user +
// last-listen date, so uq_notification prevents re-reminding for the same day.
type ListenStreakReminderJob struct {
	pool   database.DbPool
	logger *zap.Logger
	env    string
	now    func() time.Time

	mutex     sync.Mutex
	isRunning bool
}

// listenStreakBuffer is the hours-since-last-listen at which we remind (2 days
// minus 6 hours). Matches Python's LISTEN_STREAK_BUFFER.
const listenStreakBuffer = 42 * time.Hour

func NewListenStreakReminderJob(cfg config.Config, pool database.DbPool) *ListenStreakReminderJob {
	return &ListenStreakReminderJob{
		pool:   pool,
		logger: logging.NewZapLogger(cfg).Named("ListenStreakReminderJob"),
		env:    cfg.Env,
		now:    time.Now,
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *ListenStreakReminderJob) ScheduleEvery(ctx context.Context, interval time.Duration) *ListenStreakReminderJob {
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
func (j *ListenStreakReminderJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

func (j *ListenStreakReminderJob) run(ctx context.Context) error {
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
	windowEnd := now.Add(-listenStreakBuffer)
	windowStart := now.Add(-listenStreakBuffer - time.Hour)
	// Stage uses a tight 1-2 minute window so the flow can be exercised
	// end-to-end without waiting ~2 days, matching Python's env branch.
	if j.env == "stage" {
		windowEnd = now.Add(-1 * time.Minute)
		windowStart = now.Add(-2 * time.Minute)
	}

	res, err := j.pool.Exec(ctx, `
		INSERT INTO notification (specifier, group_id, blocknumber, user_ids, type, data, timestamp)
		SELECT
			cls.user_id::text,
			'listen_streak_reminder:' || cls.user_id::text || ':' || to_char(cls.last_listen_date, 'YYYY-MM-DD'),
			NULL,
			ARRAY[cls.user_id],
			'listen_streak_reminder',
			jsonb_build_object('streak', cls.listen_streak),
			@now
		FROM challenge_listen_streak cls
		WHERE cls.last_listen_date BETWEEN @window_start AND @window_end
		ON CONFLICT (group_id, specifier) DO NOTHING
	`, pgx.NamedArgs{
		"now":          now,
		"window_start": windowStart,
		"window_end":   windowEnd,
	})
	if err != nil {
		return fmt.Errorf("insert listen_streak_reminder notifications: %w", err)
	}

	j.logger.Info("Checked listen_streak_reminder notifications",
		zap.Int64("count", res.RowsAffected()),
		zap.Duration("duration", time.Since(start)))
	return nil
}
