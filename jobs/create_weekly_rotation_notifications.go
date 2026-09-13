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

// WeeklyRotationNotificationsJob tells listeners their Weekly Rotation has
// rolled over.
//
// The mix itself is computed on demand by GET /v1/users/:id/weekly-rotation
// and rolls over every Wednesday 00:00 UTC (weeklyrotation.Period), so there
// is nothing to precompute here. This job only fans out one
// `weekly_rotation` notification row per eligible listener per period; the
// pedalboard notifications app turns the row into a push (gated there by
// the push_weekly_rotation remote-config variable), and the clients render
// it in the notification feed.
//
// WHO. Anyone who has listened in the last weeklyRotationActiveWindow and
// whose account is live. Recency comes from challenge_listen_streak, which
// is one row per listener with a last_listen_date and cheap to scan, unlike
// plays. Recent listening is a proxy for "has a non-empty mix": the
// endpoint's affinity and history terms need plays to work with, and
// computing every user's mix here just to check would cost far more than
// the pushes are worth.
//
// WHEN. From weeklyRotationSendHourUTC on the Wednesday the period opens,
// for weeklyRotationSendWindow. The mix is ready at 00:00 UTC, but a push at
// 5pm Pacific on a Tuesday reads as noise; 16:00 UTC is 9am Pacific / noon
// Eastern. Someone who only becomes eligible after the window closes waits
// for the next Wednesday rather than getting "your rotation is ready" on a
// Saturday. Stage sends throughout the period so the flow can be exercised
// without waiting for a Wednesday, mirroring ListenStreakReminderJob.
//
// PACING. Each run inserts at most batchSize rows, walking user_id upward
// from a per-period cursor. Every row costs the notifications app an
// identity lookup and an SNS publish, and a single INSERT of the whole
// listener base would land there as one burst. At the scheduled interval
// this drains a few hundred thousand listeners inside the window without
// spiking anything. The cursor is in-memory: after a restart the job
// rescans from the start and the NOT EXISTS skips what was already sent.
//
// IDEMPOTENCY. group_id = weekly_rotation:<YYYY-WW>:<user_id> and specifier
// = user_id, so uq_notification (group_id, specifier) makes reruns,
// restarts and concurrent replicas safe.
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
	// weeklyRotationSendWindow is how long after that the job keeps picking
	// up newly eligible listeners.
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
		FROM challenge_listen_streak cls
		JOIN users u ON u.user_id = cls.user_id
		WHERE cls.last_listen_date >= @active_since
			AND u.user_id > @after_user_id
			AND u.is_current
			AND NOT u.is_deactivated
			AND u.is_available
			AND u.handle IS NOT NULL
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
		"active_since":  now.Add(-weeklyRotationActiveWindow),
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
