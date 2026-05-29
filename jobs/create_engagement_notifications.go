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

// EngagementNotificationsJob inserts "claimable_reward" notifications for
// challenges whose 7-day cooldown has elapsed but which haven't been disbursed
// or notified yet. Mirrors
// apps/packages/discovery-provider/src/tasks/create_engagement_notifications.py.
//
// This complements the handle_on_user_challenge DB trigger rather than
// duplicating it: that trigger emits an immediate claimable_reward only for
// cooldown_days = 0 challenges, and emits reward_in_cooldown for cooldown_days
// > 0. This job is what later promotes the 7-day-cooldown challenges to
// claimable_reward once their cooldown elapses — hence the cooldown_days = 7
// filter, matching Python's hardcoded check.
type EngagementNotificationsJob struct {
	pool   database.DbPool
	logger *zap.Logger
	now    func() time.Time

	mutex     sync.Mutex
	isRunning bool
}

// engagementStartDatetime matches Python's START_DATETIME — challenges
// completed before this date predate the cooldown-notification feature and are
// never notified.
const engagementStartDatetime = "2024-06-06"

const engagementBatchSize = 500

func NewEngagementNotificationsJob(cfg config.Config, pool database.DbPool) *EngagementNotificationsJob {
	return &EngagementNotificationsJob{
		pool:   pool,
		logger: logging.NewZapLogger(cfg).Named("EngagementNotificationsJob"),
		now:    time.Now,
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *EngagementNotificationsJob) ScheduleEvery(ctx context.Context, interval time.Duration) *EngagementNotificationsJob {
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
func (j *EngagementNotificationsJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

func (j *EngagementNotificationsJob) run(ctx context.Context) error {
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

	// completed_at < now - 1 week. The matching (group_id, specifier) dedup is
	// handled by uq_notification via ON CONFLICT; the NOT EXISTS debounce
	// suppresses a second claimable_reward for the same user within the hour
	// before completion (Python's per-row existing_notification check).
	res, err := j.pool.Exec(ctx, `
		INSERT INTO notification (specifier, group_id, blocknumber, user_ids, type, data, timestamp)
		SELECT
			uc.specifier,
			'claimable_reward:' || uc.user_id::text || ':challenge:' || uc.challenge_id || ':specifier:' || uc.specifier,
			uc.completed_blocknumber,
			ARRAY[uc.user_id],
			'claimable_reward',
			jsonb_build_object(
				'specifier', uc.specifier,
				'challenge_id', uc.challenge_id,
				'amount', uc.amount
			),
			uc.completed_at
		FROM user_challenges uc
		JOIN challenges c ON c.id = uc.challenge_id
		LEFT JOIN challenge_disbursements cd
			ON cd.challenge_id = uc.challenge_id AND cd.specifier = uc.specifier
		WHERE uc.is_complete
			AND uc.completed_at >= @start_datetime
			AND uc.completed_at < @cooldown_cutoff
			AND c.cooldown_days = 7
			AND cd.specifier IS NULL
			AND NOT EXISTS (
				SELECT 1 FROM notification n
				WHERE n.type = 'claimable_reward'
					AND n.user_ids @> ARRAY[uc.user_id]
					AND n.timestamp >= uc.completed_at - INTERVAL '1 hour'
			)
		LIMIT @batch_size
		ON CONFLICT (group_id, specifier) DO NOTHING
	`, pgx.NamedArgs{
		"start_datetime":  engagementStartDatetime,
		"cooldown_cutoff": j.now().Add(-7 * 24 * time.Hour),
		"batch_size":      engagementBatchSize,
	})
	if err != nil {
		return fmt.Errorf("insert claimable_reward notifications: %w", err)
	}

	if n := res.RowsAffected(); n > 0 {
		j.logger.Info("Inserted claimable_reward notifications", zap.Int64("count", n))
	}
	return nil
}
