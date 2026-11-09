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

// WeeklyRotationFeaturedNotificationsJob tells artists how many listeners'
// Weekly Rotations their tracks were in. One weekly_rotation_featured
// notification per artist per period; pedalboard turns the row into a push,
// gated by the push_weekly_rotation_featured remote-config variable.
//
// Mixes are computed lazily, on a listener's first request of a period, so a
// period's counts are only final once it has ended. The job therefore runs on
// rollover Wednesday and reports the period that just closed: the 2026-38
// rollover announces the 2026-37 counts. A track counts if it was in the first
// weeklyrotation.ShownSize slots of a stored mix (what clients show) and is
// still live; the rotation count is the number of distinct listeners' mixes
// that included any of the artist's tracks.
//
// Sends start at weeklyRotationFeaturedSendHourUTC on rollover day and last
// for weeklyRotationFeaturedSendWindow; stage sends all period. Each run
// inserts up to batchSize rows walking owner_id upward from an in-memory
// cursor, so pedalboard and SNS see a stream rather than a burst. Once a run
// comes back short the period is drained and later runs skip the aggregate.
// uq_notification (group_id, specifier) makes reruns, restarts and multiple
// replicas safe.
type WeeklyRotationFeaturedNotificationsJob struct {
	pool      database.DbPool
	logger    *zap.Logger
	env       string
	now       func() time.Time
	batchSize int

	mutex     sync.Mutex
	isRunning bool

	// Fan-out cursor for the period currently being reported.
	cursorPeriod  string
	cursorOwnerId int32
	drained       bool
}

const (
	// weeklyRotationFeaturedSendHourUTC is the hour (UTC) on rollover day the
	// artist fan-out begins. Two hours after the listener announcement
	// (weeklyRotationSendHourUTC) so the two fan-outs don't overlap.
	weeklyRotationFeaturedSendHourUTC = 18
	// weeklyRotationFeaturedSendWindow is how long after that the job keeps
	// sending.
	weeklyRotationFeaturedSendWindow = 24 * time.Hour
	// weeklyRotationFeaturedBatchSize is the most rows one run inserts.
	weeklyRotationFeaturedBatchSize = 1000
)

func NewWeeklyRotationFeaturedNotificationsJob(cfg config.Config, pool database.DbPool) *WeeklyRotationFeaturedNotificationsJob {
	return &WeeklyRotationFeaturedNotificationsJob{
		pool:      pool,
		logger:    logging.NewZapLogger(cfg).Named("WeeklyRotationFeaturedNotificationsJob"),
		env:       cfg.Env,
		now:       time.Now,
		batchSize: weeklyRotationFeaturedBatchSize,
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *WeeklyRotationFeaturedNotificationsJob) ScheduleEvery(ctx context.Context, interval time.Duration) *WeeklyRotationFeaturedNotificationsJob {
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
func (j *WeeklyRotationFeaturedNotificationsJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

// sendWindow returns the instants between which the period before the one
// containing `now` is reported.
func (j *WeeklyRotationFeaturedNotificationsJob) sendWindow(now time.Time) (start, end time.Time) {
	year, week := weeklyrotation.Period(now)
	periodStart := weeklyrotation.PeriodStart(year, week)
	if j.env == "stage" {
		return periodStart, periodStart.AddDate(0, 0, 7)
	}
	start = periodStart.Add(weeklyRotationFeaturedSendHourUTC * time.Hour)
	return start, start.Add(weeklyRotationFeaturedSendWindow)
}

func (j *WeeklyRotationFeaturedNotificationsJob) run(ctx context.Context) error {
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

	// The period being reported is the one that just ended.
	year, week := weeklyrotation.Period(now.AddDate(0, 0, -7))
	period := weeklyrotation.PeriodKey(year, week)
	if j.cursorPeriod != period {
		j.cursorPeriod = period
		j.cursorOwnerId = 0
		j.drained = false
	}
	if j.drained {
		return nil
	}

	// featured: one row per (listener, artist, track) in the shown slots of
	// the period's mixes, live tracks only. The aggregate is restricted to
	// owners past the cursor; LIMIT applies to the SELECT, so at most
	// batch_size rows are attempted, and RETURNING reports what landed.
	rows, err := j.pool.Query(ctx, `
		WITH featured AS (
			SELECT DISTINCT m.user_id AS listener_id, t.owner_id, t.track_id
			FROM weekly_rotation_mixes m
			CROSS JOIN LATERAL UNNEST(m.track_ids[1:@shown_size::int]) AS mix(track_id)
			JOIN tracks t ON t.track_id = mix.track_id
				AND t.is_current
				AND NOT t.is_delete
				AND NOT t.is_unlisted
				AND t.is_available
			WHERE m.period_start = @period_start::date
				AND t.owner_id > @after_owner_id
		),
		per_track AS (
			SELECT owner_id, track_id, COUNT(*) AS rotations
			FROM featured
			GROUP BY owner_id, track_id
		),
		per_artist AS (
			SELECT
				f.owner_id,
				COUNT(DISTINCT f.track_id) AS track_count,
				COUNT(DISTINCT f.listener_id) AS rotation_count,
				(
					SELECT pt.track_id FROM per_track pt
					WHERE pt.owner_id = f.owner_id
					ORDER BY pt.rotations DESC, pt.track_id
					LIMIT 1
				) AS top_track_id
			FROM featured f
			GROUP BY f.owner_id
		)
		INSERT INTO notification (specifier, group_id, blocknumber, user_ids, type, data, timestamp)
		SELECT
			a.owner_id::text,
			@group_prefix || a.owner_id::text,
			NULL,
			ARRAY[a.owner_id],
			'weekly_rotation_featured',
			jsonb_build_object(
				'year', @year::int,
				'week', @week::int,
				'track_id', a.top_track_id,
				'track_count', a.track_count,
				'rotation_count', a.rotation_count
			),
			@now
		FROM per_artist a
		JOIN users u ON u.user_id = a.owner_id
			AND u.is_current
			AND NOT u.is_deactivated
			AND u.is_available
		WHERE NOT EXISTS (
			SELECT 1 FROM notification n
			WHERE n.group_id = @group_prefix || a.owner_id::text
		)
		ORDER BY a.owner_id
		LIMIT @batch_size
		ON CONFLICT (group_id, specifier) DO NOTHING
		RETURNING user_ids[1]
	`, pgx.NamedArgs{
		"group_prefix":   "weekly_rotation_featured:" + period + ":",
		"year":           year,
		"week":           week,
		"period_start":   weeklyrotation.PeriodStart(year, week),
		"shown_size":     weeklyrotation.ShownSize,
		"now":            now,
		"after_owner_id": j.cursorOwnerId,
		"batch_size":     j.batchSize,
	})
	if err != nil {
		return fmt.Errorf("insert weekly_rotation_featured notifications: %w", err)
	}
	ownerIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return fmt.Errorf("collect weekly_rotation_featured notifications: %w", err)
	}
	for _, id := range ownerIds {
		if id > j.cursorOwnerId {
			j.cursorOwnerId = id
		}
	}
	// A short batch means nothing past the cursor was left to send. (A
	// conflict with another replica can also shorten it; that replica then
	// covers the rest.)
	if len(ownerIds) < j.batchSize {
		j.drained = true
	}

	if len(ownerIds) > 0 {
		j.logger.Info("Inserted weekly_rotation_featured notifications",
			zap.String("period", period),
			zap.Int("count", len(ownerIds)),
			zap.Int32("cursor", j.cursorOwnerId),
			zap.Duration("duration", time.Since(start)))
	}
	return nil
}
