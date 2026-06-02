package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	dbv1 "api.audius.co/api/dbv1"
	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type UserSocialSetsJob struct {
	pool      database.DbPool
	logger    *zap.Logger
	batchSize int

	mutex     sync.Mutex
	isRunning bool
}

const UserSocialSetsBatchSize = 50

type dirtySocialSet struct {
	UserID    int32
	UpdatedAt time.Time
}

func NewUserSocialSetsJob(cfg config.Config, pool database.DbPool) *UserSocialSetsJob {
	return &UserSocialSetsJob{
		pool:      pool,
		logger:    logging.NewZapLogger(cfg).Named("UserSocialSetsJob"),
		batchSize: UserSocialSetsBatchSize,
	}
}

func (j *UserSocialSetsJob) WithBatchSize(n int) *UserSocialSetsJob {
	j.batchSize = n
	return j
}

func (j *UserSocialSetsJob) ScheduleEvery(ctx context.Context, interval time.Duration) *UserSocialSetsJob {
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

func (j *UserSocialSetsJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

func (j *UserSocialSetsJob) run(ctx context.Context) error {
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

	selected, err := j.selectDirty(ctx)
	if err != nil {
		return err
	}
	if len(selected) < j.batchSize {
		missing, err := j.selectMissing(ctx, j.batchSize-len(selected))
		if err != nil {
			return err
		}
		selected = append(selected, missing...)
	}
	if len(selected) == 0 {
		return nil
	}

	queries := dbv1.New(j.pool)
	rebuilt := 0
	for _, row := range selected {
		if err := queries.RebuildUserSocialSet(ctx, row.UserID); err != nil {
			return fmt.Errorf("rebuild social set for user %d: %w", row.UserID, err)
		}
		rebuilt++
		if !row.UpdatedAt.IsZero() {
			if _, err := j.pool.Exec(ctx, `
				DELETE FROM user_social_set_dirty
				WHERE user_id = @user_id
				  AND updated_at <= @updated_at
			`, pgx.NamedArgs{
				"user_id":    row.UserID,
				"updated_at": row.UpdatedAt,
			}); err != nil {
				return err
			}
		}
	}

	j.logger.Info("Rebuilt user social sets", zap.Int("count", rebuilt))
	return nil
}

func (j *UserSocialSetsJob) selectDirty(ctx context.Context) ([]dirtySocialSet, error) {
	rows, err := j.pool.Query(ctx, `
		SELECT user_id, updated_at
		FROM user_social_set_dirty
		ORDER BY updated_at ASC
		LIMIT @limit
	`, pgx.NamedArgs{"limit": j.batchSize})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	selected := []dirtySocialSet{}
	for rows.Next() {
		var row dirtySocialSet
		if err := rows.Scan(&row.UserID, &row.UpdatedAt); err != nil {
			return nil, err
		}
		selected = append(selected, row)
	}
	return selected, rows.Err()
}

func (j *UserSocialSetsJob) selectMissing(ctx context.Context, limit int) ([]dirtySocialSet, error) {
	if limit <= 0 {
		return nil, nil
	}

	rows, err := j.pool.Query(ctx, `
		SELECT u.user_id
		FROM users u
		JOIN aggregate_user au USING (user_id)
		LEFT JOIN user_social_sets uss USING (user_id)
		LEFT JOIN user_social_set_dirty dirty USING (user_id)
		WHERE u.is_current = true
		  AND uss.user_id IS NULL
		  AND dirty.user_id IS NULL
		  AND (au.following_count > 0 OR au.follower_count > 0)
		ORDER BY u.user_id
		LIMIT @limit
	`, pgx.NamedArgs{"limit": limit})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	selected := []dirtySocialSet{}
	for rows.Next() {
		var userID int32
		if err := rows.Scan(&userID); err != nil {
			return nil, err
		}
		selected = append(selected, dirtySocialSet{UserID: userID})
	}
	return selected, rows.Err()
}
