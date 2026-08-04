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

// RemixContestNotificationsJob emits the four time-based remix-contest
// notifications. Mirrors
// apps/packages/discovery-provider/src/tasks/create_remix_contest_notifications.py
// and its four sub-tasks under remix_contest_notifications/.
//
// All four select remix_contest events whose end_date falls in a window
// relative to now, guard that the contest's parent track is still
// current/visible, and insert one notification per recipient. uq_notification
// (group_id, specifier) — where specifier is the recipient user id — provides
// idempotency, so a recipient is notified at most once per (event, type).
//
// Each step also filters out events that already have any notification row for
// their group_id prefix. This prevents Postgres from burning a sequence ID on
// every conflict when the job re-scans the same events each run — without this
// guard, a 72-hour ending-soon window and a 30-second job interval causes
// ~8 640 wasted INSERT attempts per event per day.
type RemixContestNotificationsJob struct {
	pool   database.DbPool
	logger *zap.Logger
	now    func() time.Time

	mutex     sync.Mutex
	isRunning bool
}

const (
	remixContestEndedWindowHours       = 24 * time.Hour
	fanRemixContestEndingSoonWindow    = 72 * time.Hour
	artistRemixContestEndingSoonWindow = 48 * time.Hour
)

func NewRemixContestNotificationsJob(cfg config.Config, pool database.DbPool) *RemixContestNotificationsJob {
	return &RemixContestNotificationsJob{
		pool:   pool,
		logger: logging.NewZapLogger(cfg).Named("RemixContestNotificationsJob"),
		now:    time.Now,
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *RemixContestNotificationsJob) ScheduleEvery(ctx context.Context, interval time.Duration) *RemixContestNotificationsJob {
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
func (j *RemixContestNotificationsJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

// RunE executes the job once and returns any error.
// Intended for the cmd/remix_contest_notifications CronJob binary.
func (j *RemixContestNotificationsJob) RunE(ctx context.Context) error {
	return j.run(ctx)
}

func (j *RemixContestNotificationsJob) run(ctx context.Context) error {
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

	// Each sub-step is independent; a failure in one shouldn't block the
	// others, mirroring the Python dispatcher's per-call isolation.
	steps := []struct {
		name string
		fn   func(context.Context, time.Time) (int64, error)
	}{
		{"fan_remix_contest_ended", j.fanEnded},
		{"fan_remix_contest_ending_soon", j.fanEndingSoon},
		{"artist_remix_contest_ended", j.artistEnded},
		{"artist_remix_contest_ending_soon", j.artistEndingSoon},
	}
	var firstErr error
	for _, s := range steps {
		stepStart := time.Now()
		n, err := s.fn(ctx, now)
		if err != nil {
			j.logger.Error("remix contest notification step failed",
				zap.String("step", s.name),
				zap.Duration("duration", time.Since(stepStart)),
				zap.Error(err))
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if n > 0 {
			j.logger.Info("Inserted remix contest notifications",
				zap.String("step", s.name),
				zap.Int64("count", n),
				zap.Duration("duration", time.Since(stepStart)))
		} else {
			j.logger.Debug("No remix contest notifications inserted",
				zap.String("step", s.name),
				zap.Duration("duration", time.Since(stepStart)))
		}
	}
	j.logger.Info("Checked remix contest notifications",
		zap.Duration("duration", time.Since(start)))
	return firstErr
}

// fanEnded notifies remixers, event subscribers, host followers, and parent
// track favoriters (excluding the host) that a contest they engaged with ended
// in the last 24h.
func (j *RemixContestNotificationsJob) fanEnded(ctx context.Context, now time.Time) (int64, error) {
	res, err := j.pool.Exec(ctx, `
		INSERT INTO notification (specifier, group_id, blocknumber, user_ids, type, data, timestamp)
		SELECT
			aud.user_id::text,
			'fan_remix_contest_ended:' || e.event_id::text,
			NULL,
			ARRAY[aud.user_id],
			'fan_remix_contest_ended',
			jsonb_build_object('entity_id', e.entity_id, 'entity_user_id', pt.owner_id),
			@now
		FROM events e
		JOIN tracks pt ON pt.track_id = e.entity_id
			AND pt.is_current AND NOT pt.is_delete AND NOT pt.is_unlisted
		CROSS JOIN LATERAL (
			SELECT t.owner_id AS user_id FROM tracks t
				WHERE t.is_current AND NOT t.is_delete AND t.remix_of IS NOT NULL
					AND t.remix_of->'tracks' @> jsonb_build_array(jsonb_build_object('parent_track_id', e.entity_id))
			UNION
			SELECT s.subscriber_id FROM subscriptions s
				WHERE s.entity_id = e.event_id AND s.entity_type = 'Event'
					AND s.is_current AND NOT s.is_delete
			UNION
			SELECT f.follower_user_id FROM follows f
				WHERE f.followee_user_id = e.user_id AND f.is_current AND NOT f.is_delete
			UNION
			SELECT sv.user_id FROM saves sv
				WHERE sv.save_item_id = e.entity_id AND sv.save_type = 'track'
					AND sv.is_current AND NOT sv.is_delete
		) aud
		WHERE e.event_type = 'remix_contest' AND NOT e.is_deleted
			AND e.end_date IS NOT NULL
			AND e.end_date BETWEEN @window_start AND @window_end
			AND aud.user_id <> e.user_id
				AND NOT EXISTS (
			SELECT 1 FROM notification
			WHERE group_id = 'fan_remix_contest_ended:' || e.event_id::text
		)
ON CONFLICT (group_id, specifier) DO NOTHING
	`, pgx.NamedArgs{
		"now":          now,
		"window_start": now.Add(-remixContestEndedWindowHours),
		"window_end":   now,
	})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}

// fanEndingSoon notifies host followers, parent track favoriters, and event
// subscribers (excluding the host) that a contest ends within 72h.
func (j *RemixContestNotificationsJob) fanEndingSoon(ctx context.Context, now time.Time) (int64, error) {
	res, err := j.pool.Exec(ctx, `
		INSERT INTO notification (specifier, group_id, blocknumber, user_ids, type, data, timestamp)
		SELECT
			aud.user_id::text,
			'fan_remix_contest_ending_soon:' || e.event_id::text,
			NULL,
			ARRAY[aud.user_id],
			'fan_remix_contest_ending_soon',
			jsonb_build_object('entity_id', e.entity_id, 'entity_user_id', pt.owner_id),
			@now
		FROM events e
		JOIN tracks pt ON pt.track_id = e.entity_id
			AND pt.is_current AND NOT pt.is_delete AND NOT pt.is_unlisted
		CROSS JOIN LATERAL (
			SELECT f.follower_user_id AS user_id FROM follows f
				WHERE f.followee_user_id = e.user_id AND f.is_current AND NOT f.is_delete
			UNION
			SELECT sv.user_id FROM saves sv
				WHERE sv.save_item_id = e.entity_id AND sv.save_type = 'track'
					AND sv.is_current AND NOT sv.is_delete
			UNION
			SELECT s.subscriber_id FROM subscriptions s
				WHERE s.entity_id = e.event_id AND s.entity_type = 'Event'
					AND s.is_current AND NOT s.is_delete
		) aud
		WHERE e.event_type = 'remix_contest' AND NOT e.is_deleted
			AND e.end_date IS NOT NULL
			AND e.end_date BETWEEN @window_start AND @window_end
			AND aud.user_id <> e.user_id
				AND NOT EXISTS (
			SELECT 1 FROM notification
			WHERE group_id = 'fan_remix_contest_ending_soon:' || e.event_id::text
		)
ON CONFLICT (group_id, specifier) DO NOTHING
	`, pgx.NamedArgs{
		"now":          now,
		"window_start": now,
		"window_end":   now.Add(fanRemixContestEndingSoonWindow),
	})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}

// artistEnded notifies the contest host that their contest ended in the last 24h.
func (j *RemixContestNotificationsJob) artistEnded(ctx context.Context, now time.Time) (int64, error) {
	res, err := j.pool.Exec(ctx, `
		INSERT INTO notification (specifier, group_id, blocknumber, user_ids, type, data, timestamp)
		SELECT
			e.user_id::text,
			'artist_remix_contest_ended:' || e.event_id::text,
			NULL,
			ARRAY[e.user_id],
			'artist_remix_contest_ended',
			jsonb_build_object('entity_id', e.entity_id),
			@now
		FROM events e
		JOIN tracks pt ON pt.track_id = e.entity_id
			AND pt.is_current AND NOT pt.is_delete AND NOT pt.is_unlisted
		WHERE e.event_type = 'remix_contest' AND NOT e.is_deleted
			AND e.end_date IS NOT NULL
			AND e.end_date BETWEEN @window_start AND @window_end
				AND NOT EXISTS (
			SELECT 1 FROM notification
			WHERE group_id = 'artist_remix_contest_ended:' || e.event_id::text
		)
ON CONFLICT (group_id, specifier) DO NOTHING
	`, pgx.NamedArgs{
		"now":          now,
		"window_start": now.Add(-remixContestEndedWindowHours),
		"window_end":   now,
	})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}

// artistEndingSoon notifies the contest host that their contest ends within 48h.
func (j *RemixContestNotificationsJob) artistEndingSoon(ctx context.Context, now time.Time) (int64, error) {
	res, err := j.pool.Exec(ctx, `
		INSERT INTO notification (specifier, group_id, blocknumber, user_ids, type, data, timestamp)
		SELECT
			e.user_id::text,
			'artist_remix_contest_ending_soon:' || e.event_id::text,
			NULL,
			ARRAY[e.user_id],
			'artist_remix_contest_ending_soon',
			jsonb_build_object('entity_id', e.entity_id, 'entity_user_id', pt.owner_id),
			@now
		FROM events e
		JOIN tracks pt ON pt.track_id = e.entity_id
			AND pt.is_current AND NOT pt.is_delete AND NOT pt.is_unlisted
		WHERE e.event_type = 'remix_contest' AND NOT e.is_deleted
			AND e.end_date IS NOT NULL
			AND e.end_date BETWEEN @window_start AND @window_end
				AND NOT EXISTS (
			SELECT 1 FROM notification
			WHERE group_id = 'artist_remix_contest_ending_soon:' || e.event_id::text
		)
ON CONFLICT (group_id, specifier) DO NOTHING
	`, pgx.NamedArgs{
		"now":          now,
		"window_start": now,
		"window_end":   now.Add(artistRemixContestEndingSoonWindow),
	})
	if err != nil {
		return 0, err
	}
	return res.RowsAffected(), nil
}
