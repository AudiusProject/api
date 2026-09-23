package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"api.audius.co/weeklyrotation"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// WeeklyRotationNotificationsJob inserts one weekly_rotation notification per
// recent listener per period (weeklyrotation.Period); pedalboard turns the row
// into a push, gated by the push_weekly_rotation remote-config variable.
//
// Recipients are live users with a play in the last weeklyRotationActiveWindow.
// Sends start at weeklyRotationSendHourUTC on rollover Wednesday and last for
// weeklyRotationSendWindow; stage sends all period. Each run inserts up to
// batchSize rows, walking user_id upward from an in-memory cursor, so one pass
// covers the listeners eligible when it reaches them. uq_notification
// (group_id, specifier) makes reruns, restarts and multiple replicas safe.
type WeeklyRotationNotificationsJob struct {
	pool      database.DbPool
	logger    *zap.Logger
	env       string
	now       func() time.Time
	batchSize int

	mutex     sync.Mutex
	isRunning bool

	// Fan-out cursor for the period currently being sent.
	cursorPeriod string
	cursorUserId int32
}

const (
	// weeklyRotationSendHourUTC is the hour (UTC) on rollover day the fan-out
	// begins.
	weeklyRotationSendHourUTC = 16
	// weeklyRotationSendWindow is how long after that the job keeps sending.
	weeklyRotationSendWindow = 24 * time.Hour
	// weeklyRotationActiveWindow is how recently someone must have listened
	// to be told about their mix.
	weeklyRotationActiveWindow = 30 * 24 * time.Hour
	// weeklyRotationBatchSize is the most rows one run inserts.
	weeklyRotationBatchSize = 1000
)

func NewWeeklyRotationNotificationsJob(cfg config.Config, pool database.DbPool) *WeeklyRotationNotificationsJob {
	return &WeeklyRotationNotificationsJob{
		pool:      pool,
		logger:    logging.NewZapLogger(cfg).Named("WeeklyRotationNotificationsJob"),
		env:       cfg.Env,
		now:       time.Now,
		batchSize: weeklyRotationBatchSize,
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *WeeklyRotationNotificationsJob) ScheduleEvery(ctx context.Context, interval time.Duration) *WeeklyRotationNotificationsJob {
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
func (j *WeeklyRotationNotificationsJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

// sendWindow returns the instants between which the period containing
// `now` is announced.
func (j *WeeklyRotationNotificationsJob) sendWindow(now time.Time) (start, end time.Time) {
	year, week := weeklyrotation.Period(now)
	periodStart := weeklyrotation.PeriodStart(year, week)
	if j.env == "stage" {
		return periodStart, periodStart.AddDate(0, 0, 7)
	}
	start = periodStart.Add(weeklyRotationSendHourUTC * time.Hour)
	return start, start.Add(weeklyRotationSendWindow)
}

func (j *WeeklyRotationNotificationsJob) run(ctx context.Context) error {
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
	windowStart, windowEnd := j.sendWindow(now)
	if now.Before(windowStart) || !now.Before(windowEnd) {
		return nil
	}

	year, week := weeklyrotation.Period(now)
	period := weeklyrotation.PeriodKey(year, week)
	if j.cursorPeriod != period {
		j.cursorPeriod = period
		j.cursorUserId = 0
	}

	// LIMIT applies to the SELECT, so at most batch_size rows are attempted;
	// RETURNING reports what actually landed so the cursor can advance.
	rows, err := j.pool.Query(ctx, `
		INSERT INTO notification (specifier, group_id, blocknumber, user_ids, type, data, timestamp)
		SELECT
			u.user_id::text,
			@group_prefix || u.user_id::text,
			NULL,
			ARRAY[u.user_id],
			'weekly_rotation',
			jsonb_build_object('year', @year::int, 'week', @week::int),
			@now
		FROM users u
		WHERE u.user_id > @after_user_id
			AND u.is_current
			AND NOT u.is_deactivated
			AND u.is_available
			AND u.handle IS NOT NULL
			-- Matches ix_plays_user_hour's expression so this is an index range scan.
			AND EXISTS (
				SELECT 1 FROM plays p
				WHERE p.user_id = u.user_id
					AND date_trunc('hour', p.created_at) >= date_trunc('hour', @active_since::timestamp)
			)
			AND NOT EXISTS (
				SELECT 1 FROM notification n
				WHERE n.group_id = @group_prefix || u.user_id::text
			)
		ORDER BY u.user_id
		LIMIT @batch_size
		ON CONFLICT (group_id, specifier) DO NOTHING
		RETURNING user_ids[1]
	`, pgx.NamedArgs{
		"group_prefix":  "weekly_rotation:" + period + ":",
		"year":          year,
		"week":          week,
		"now":           now,
		"active_since":  now.Add(-weeklyRotationActiveWindow).UTC(),
		"after_user_id": j.cursorUserId,
		"batch_size":    j.batchSize,
	})
	if err != nil {
		return fmt.Errorf("insert weekly_rotation notifications: %w", err)
	}
	userIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return fmt.Errorf("collect weekly_rotation notifications: %w", err)
	}
	for _, id := range userIds {
		if id > j.cursorUserId {
			j.cursorUserId = id
		}
	}

	if len(userIds) > 0 {
		j.logger.Info("Inserted weekly_rotation notifications",
			zap.String("period", period),
			zap.Int("count", len(userIds)),
			zap.Int32("cursor", j.cursorUserId),
			zap.Duration("duration", time.Since(start)))
	}
	return nil
}
