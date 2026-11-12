package jobs

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"api.audius.co/weeklyrotation"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// WeeklyArtistRecapNotificationsJob sends each artist a recap of the week
// that just closed: plays, new followers and saves, plus
// how many of those saves came from listeners whose Weekly Rotation included
// the track. One weekly_artist_recap notification per artist per week;
// pedalboard turns the row into a push, gated by the push_weekly_artist_recap
// remote-config variable. Artists with no plays that week get nothing.
//
// The week is the Weekly Rotation period (Wednesday 00:00 UTC to the next
// Wednesday), and timing matches WeeklyRotationFeaturedNotificationsJob: sends
// start at weeklyArtistRecapSendHourUTC on rollover day, last for
// weeklyArtistRecapSendWindow, and report the period that just closed. Stage
// sends all period.
//
// Counting a week of plays is the expensive part, so each replica does it once
// per period and keeps the result in memory sorted by artist. Each run then
// inserts up to batchSize rows past an owner_id cursor, so pedalboard and SNS
// see a stream rather than a burst. uq_notification (group_id, specifier) plus
// NOT EXISTS makes reruns, restarts and multiple replicas safe.
type WeeklyArtistRecapNotificationsJob struct {
	pool      database.DbPool
	logger    *zap.Logger
	env       string
	now       func() time.Time
	batchSize int

	mutex     sync.Mutex
	isRunning bool

	cursorPeriod  string
	cursorOwnerId int32
	recaps        []artistRecap
	loaded        bool
	drained       bool
}

type artistRecap struct {
	OwnerId           int32
	PlayCount         int64
	FollowerCount     int64
	SaveCount         int64
	RotationSaveCount int64
}

const (
	// weeklyArtistRecapSendHourUTC is the hour (UTC) on rollover day the
	// fan-out begins, alongside the weekly_rotation_featured fan-out.
	weeklyArtistRecapSendHourUTC = 18
	// weeklyArtistRecapSendWindow is how long after that the job keeps sending.
	weeklyArtistRecapSendWindow = 24 * time.Hour
	// weeklyArtistRecapBatchSize is the most rows one run inserts.
	weeklyArtistRecapBatchSize = 1000
)

func NewWeeklyArtistRecapNotificationsJob(cfg config.Config, pool database.DbPool) *WeeklyArtistRecapNotificationsJob {
	return &WeeklyArtistRecapNotificationsJob{
		pool:      pool,
		logger:    logging.NewZapLogger(cfg).Named("WeeklyArtistRecapNotificationsJob"),
		env:       cfg.Env,
		now:       time.Now,
		batchSize: weeklyArtistRecapBatchSize,
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *WeeklyArtistRecapNotificationsJob) ScheduleEvery(ctx context.Context, interval time.Duration) *WeeklyArtistRecapNotificationsJob {
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
func (j *WeeklyArtistRecapNotificationsJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

// sendWindow returns the instants between which the period before the one
// containing `now` is reported.
func (j *WeeklyArtistRecapNotificationsJob) sendWindow(now time.Time) (start, end time.Time) {
	year, week := weeklyrotation.Period(now)
	periodStart := weeklyrotation.PeriodStart(year, week)
	if j.env == "stage" {
		return periodStart, periodStart.AddDate(0, 0, 7)
	}
	start = periodStart.Add(weeklyArtistRecapSendHourUTC * time.Hour)
	return start, start.Add(weeklyArtistRecapSendWindow)
}

func (j *WeeklyArtistRecapNotificationsJob) run(ctx context.Context) error {
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
		j.recaps = nil
		j.loaded = false
		j.drained = false
	}
	if j.drained {
		return nil
	}

	periodStart := weeklyrotation.PeriodStart(year, week)
	if !j.loaded {
		recaps, err := j.loadRecaps(ctx, periodStart)
		if err != nil {
			return err
		}
		j.recaps = recaps
		j.loaded = true
		j.logger.Info("Loaded weekly_artist_recap counts",
			zap.String("period", period),
			zap.Int("artists", len(recaps)),
			zap.Duration("duration", time.Since(start)))
	}

	// Next batch past the cursor.
	i := sort.Search(len(j.recaps), func(i int) bool { return j.recaps[i].OwnerId > j.cursorOwnerId })
	batch := j.recaps[i:min(i+j.batchSize, len(j.recaps))]
	if len(batch) == 0 {
		j.drained = true
		j.recaps = nil
		return nil
	}

	ownerIds := make([]int32, len(batch))
	plays := make([]int64, len(batch))
	followers := make([]int64, len(batch))
	saves := make([]int64, len(batch))
	rotationSaves := make([]int64, len(batch))
	for k, r := range batch {
		ownerIds[k] = r.OwnerId
		plays[k] = r.PlayCount
		followers[k] = r.FollowerCount
		saves[k] = r.SaveCount
		rotationSaves[k] = r.RotationSaveCount
	}

	tag, err := j.pool.Exec(ctx, `
		INSERT INTO notification (specifier, group_id, blocknumber, user_ids, type, data, timestamp)
		SELECT
			r.owner_id::text,
			@group_prefix || r.owner_id::text,
			NULL,
			ARRAY[r.owner_id],
			'weekly_artist_recap',
			jsonb_strip_nulls(jsonb_build_object(
				'year', @year::int,
				'week', @week::int,
				'play_count', r.play_count,
				'follower_count', r.follower_count,
				'save_count', r.save_count,
				'rotation_save_count', r.rotation_save_count
			)),
			@now
		FROM UNNEST(
			@owner_ids::int[], @plays::bigint[], @followers::bigint[],
			@saves::bigint[], @rotation_saves::bigint[]
		) AS r(owner_id, play_count, follower_count, save_count, rotation_save_count)
		WHERE NOT EXISTS (
			SELECT 1 FROM notification n
			WHERE n.group_id = @group_prefix || r.owner_id::text
		)
		ON CONFLICT (group_id, specifier) DO NOTHING
	`, pgx.NamedArgs{
		"group_prefix":   "weekly_artist_recap:" + period + ":",
		"year":           year,
		"week":           week,
		"now":            now,
		"owner_ids":      ownerIds,
		"plays":          plays,
		"followers":      followers,
		"saves":          saves,
		"rotation_saves": rotationSaves,
	})
	if err != nil {
		return fmt.Errorf("insert weekly_artist_recap notifications: %w", err)
	}
	j.cursorOwnerId = batch[len(batch)-1].OwnerId
	if len(batch) < j.batchSize {
		j.drained = true
		j.recaps = nil
	}

	if tag.RowsAffected() > 0 {
		j.logger.Info("Inserted weekly_artist_recap notifications",
			zap.String("period", period),
			zap.Int64("count", tag.RowsAffected()),
			zap.Int32("cursor", j.cursorOwnerId),
			zap.Duration("duration", time.Since(start)))
	}
	return nil
}

// loadRecaps counts the period starting at periodStart for every artist whose
// tracks were played by someone else that week, skipping deactivated and
// unavailable artists. Sorted by owner_id.
func (j *WeeklyArtistRecapNotificationsJob) loadRecaps(ctx context.Context, periodStart time.Time) ([]artistRecap, error) {
	rows, err := j.pool.Query(ctx, `
		WITH week_plays AS (
			SELECT t.owner_id
			FROM plays p
			JOIN tracks t ON t.track_id = p.play_item_id AND t.is_current
			WHERE p.created_at >= @start AND p.created_at < @end
				AND p.user_id IS DISTINCT FROM t.owner_id
		),
		per_artist AS (
			SELECT owner_id, COUNT(*) AS play_count
			FROM week_plays
			GROUP BY owner_id
		),
		artists AS (
			SELECT a.owner_id, a.play_count
			FROM per_artist a
			JOIN users u ON u.user_id = a.owner_id
				AND u.is_current
				AND NOT u.is_deactivated
				AND u.is_available
		),
		new_followers AS (
			SELECT f.followee_user_id AS owner_id, COUNT(*) AS follower_count
			FROM follows f
			JOIN artists a ON a.owner_id = f.followee_user_id
			WHERE f.is_current AND NOT f.is_delete
				AND f.created_at >= @start AND f.created_at < @end
			GROUP BY f.followee_user_id
		),
		week_saves AS (
			SELECT t.owner_id, s.user_id, s.save_item_id
			FROM saves s
			JOIN tracks t ON t.track_id = s.save_item_id AND t.is_current
			JOIN artists a ON a.owner_id = t.owner_id
			WHERE s.save_type = 'track'
				AND s.is_current AND NOT s.is_delete
				AND s.created_at >= @start AND s.created_at < @end
		),
		save_counts AS (
			SELECT
				ws.owner_id,
				COUNT(*) AS save_count,
				COUNT(*) FILTER (WHERE EXISTS (
					SELECT 1 FROM weekly_rotation_mixes m
					WHERE m.user_id = ws.user_id
						AND m.period_start = @start::date
						AND ws.save_item_id = ANY(m.track_ids[1:@shown_size::int])
				)) AS rotation_save_count
			FROM week_saves ws
			GROUP BY ws.owner_id
		)
		SELECT
			a.owner_id,
			a.play_count,
			COALESCE(nf.follower_count, 0),
			COALESCE(sc.save_count, 0),
			COALESCE(sc.rotation_save_count, 0)
		FROM artists a
		LEFT JOIN new_followers nf ON nf.owner_id = a.owner_id
		LEFT JOIN save_counts sc ON sc.owner_id = a.owner_id
		ORDER BY a.owner_id
	`, pgx.NamedArgs{
		"start":      periodStart,
		"end":        periodStart.AddDate(0, 0, 7),
		"shown_size": weeklyrotation.ShownSize,
	})
	if err != nil {
		return nil, fmt.Errorf("count weekly_artist_recap: %w", err)
	}
	recaps, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (artistRecap, error) {
		var r artistRecap
		err := row.Scan(&r.OwnerId, &r.PlayCount, &r.FollowerCount, &r.SaveCount, &r.RotationSaveCount)
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect weekly_artist_recap: %w", err)
	}
	return recaps, nil
}
