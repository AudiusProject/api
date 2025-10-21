package jobs

import (
	"context"
	"strings"
	"time"

	dbv1 "api.audius.co/api/dbv1"
	"api.audius.co/config"
	"api.audius.co/database"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type UpdateAggregatesJobRunner struct {
	writePool database.DbPool
	readPool  *dbv1.DBPools
}

const (
	BatchSize = 10000
)

func (r *UpdateAggregatesJobRunner) Execute(ctx context.Context, logger *zap.Logger) error {
	startTime := time.Now()
	logger.Info("Starting user score update job")

	type UserScoreRecord struct {
		UserID    int32
		CreatedAt time.Time
		Score     int64
	}

	var lastUserID *int32
	var lastCreatedAt *time.Time
	processedCount := 0
	scoreUpdatedCount := int64(0)

	for {
		filters := []string{
			"u.is_current = TRUE",
			"u.handle_lc IS NOT NULL",
		}
		if lastUserID != nil && lastCreatedAt != nil {
			filters = append(filters, `((u.created_at, u.user_id) < (@cursorTime::timestamptz, @cursorUserId::int))`)
			logger.Info("Processing batch", zap.String("lastCreatedAt", lastCreatedAt.Format(time.RFC3339)), zap.Int32("lastUserID", *lastUserID))
		} else {
			logger.Info("Processing first batch")
		}

		query := `
			WITH batch AS MATERIALIZED (
			SELECT u.user_id, u.created_at
			FROM users u
			WHERE ` + strings.Join(filters, " AND ") + `
			ORDER BY u.created_at DESC, u.user_id DESC
			LIMIT @batchSize
			),
			ids AS MATERIALIZED (
			SELECT array_agg(user_id) AS ids FROM batch
			),

			/* plays: split into two small per-user aggregates */
			hours_agg AS (
			SELECT h.user_id, COUNT(*)::bigint AS play_hours
			FROM (
				SELECT DISTINCT p.user_id, date_trunc('hour', p.created_at) AS hr
				FROM plays p, ids
				WHERE p.user_id = ANY(ids.ids)
			) h
			GROUP BY h.user_id
			),
			tracks_agg AS (
			SELECT p.user_id, COUNT(DISTINCT p.play_item_id)::bigint AS distinct_tracks
			FROM plays p, ids
			WHERE p.user_id = ANY(ids.ids)
			GROUP BY p.user_id
			),

			fast_challenge_completion AS (
			SELECT b.user_id, COUNT(*)::bigint AS challenge_count
			FROM batch b
			JOIN user_challenges uc
				ON uc.user_id = b.user_id
			AND uc.is_complete
			AND uc.challenge_id NOT IN ('m','b')
			AND uc.completed_at <= (b.created_at + interval '3 minutes')
			GROUP BY b.user_id
			),
			chat_blocks AS (
			SELECT b.user_id, COUNT(*)::bigint AS block_count
			FROM batch b
			JOIN chat_blocked_users c ON c.blockee_user_id = b.user_id
			GROUP BY b.user_id
			),
			followers_karma AS (
			SELECT b.user_id,
					LEAST((SUM(fau.follower_count) / 100)::bigint, 100::bigint) AS karma_sum
			FROM batch b
			JOIN follows f
				ON f.followee_user_id = b.user_id
			AND f.is_delete = FALSE
			JOIN aggregate_user fau
				ON fau.user_id = f.follower_user_id
			AND fau.following_count < 10000
			GROUP BY b.user_id
			),

			/* compute features for scoring */
			features AS (
			SELECT
				u.user_id,
				b.created_at,
				COALESCE(h.play_hours,       0)::bigint AS play_count,             -- distinct hours
				COALESCE(t.distinct_tracks,  0)::bigint AS distinct_tracks_played, -- distinct tracks
				COALESCE(c.challenge_count,  0)::bigint AS challenge_count,
				COALESCE(au.following_count, 0)::bigint AS following_count,
				COALESCE(au.follower_count,  0)::bigint AS follower_count,
				COALESCE(cb.block_count,     0)::bigint AS chat_block_count,
				( ((u.handle_lc ILIKE '%audius%') OR (lower(u.name) ILIKE '%audius%'))
				AND u.is_verified = FALSE )               AS is_audius_impersonator,
				( u.is_verified = FALSE
				AND (u.handle_lc ILIKE '%airdrop%' OR lower(u.name) LIKE '%airdrop%') )
															AS has_badwords,
				CASE
				WHEN COALESCE(au.follower_count, 0) > 1000 THEN 100
				WHEN COALESCE(au.follower_count, 0) = 0     THEN 0
				ELSE COALESCE(k.karma_sum, 0)
				END::bigint                                  AS karma
			FROM batch b
			JOIN users u               ON u.user_id = b.user_id
			LEFT JOIN hours_agg   h    ON h.user_id = b.user_id
			LEFT JOIN tracks_agg  t    ON t.user_id = b.user_id
			LEFT JOIN fast_challenge_completion c ON c.user_id = b.user_id
			LEFT JOIN chat_blocks     cb ON cb.user_id = b.user_id
			LEFT JOIN aggregate_user  au ON au.user_id = b.user_id
			LEFT JOIN followers_karma k  ON k.user_id  = b.user_id
			)

			SELECT
			f.user_id,
			f.created_at,
			compute_user_score(
				f.play_count,
				f.follower_count,
				f.challenge_count,
				f.chat_block_count,
				f.following_count,
				f.is_audius_impersonator,
				f.has_badwords,
				f.distinct_tracks_played,
				f.karma
			) AS score
			FROM features f
			ORDER BY f.created_at DESC, f.user_id DESC
		`

		readQueryStart := time.Now()
		res, err := r.readPool.Query(ctx, query, pgx.NamedArgs{
			"batchSize":    BatchSize,
			"cursorTime":   lastCreatedAt,
			"cursorUserId": lastUserID,
		})
		if err != nil {
			logger.Error("Failed to execute batch read query", zap.Error(err))
			return err
		}
		readQueryDuration := time.Since(readQueryStart)

		userIDs := make([]int32, 0)
		scores := make([]int64, 0)
		seenUserIDs := make(map[int32]bool)
		createdAt := time.Time{}
		score := int64(0)
		userID := int32(0)
		fetchedRows := 0
		pgx.ForEachRow(res, []any{&userID, &createdAt, &score}, func() error {
			fetchedRows++
			// We get dupes on some user_ids due to multiple is_current user rows.
			// Upsert query will fail if we don't remove them.
			if !seenUserIDs[userID] {
				userIDs = append(userIDs, userID)
				scores = append(scores, score)
				seenUserIDs[userID] = true
			}
			return nil
		})
		res.Close()

		lastCreatedAt = &createdAt
		lastUserID = &userID

		writeQueryStart := time.Now()
		tag, err := r.writePool.Exec(ctx, `
		WITH s AS (
		SELECT * FROM unnest($1::bigint[], $2::double precision[]) AS t(user_id, score)
		)
		INSERT INTO aggregate_user (user_id, score)
		SELECT s.user_id, s.score
		FROM s
		ON CONFLICT (user_id)
		DO UPDATE SET
			score = EXCLUDED.score
		WHERE aggregate_user.score IS DISTINCT FROM EXCLUDED.score
		`, userIDs, scores)
		writeQueryDuration := time.Since(writeQueryStart)
		if err != nil {
			logger.Error("Failed to execute update query", zap.Error(err))
			return err
		}

		processedCount += fetchedRows
		scoreUpdatedCount += tag.RowsAffected()
		logger.Info("Processed batch",
			zap.Int("batch_size", fetchedRows),
			zap.Int32("last_user_id", userID),
			zap.String("last_created_at", lastCreatedAt.Format(time.RFC3339)),
			zap.Int("total_processed", processedCount),
			zap.Int64("total_scores_changes", tag.RowsAffected()),
			zap.Duration("read_query_duration", readQueryDuration),
			zap.Duration("write_query_duration", writeQueryDuration))

		if fetchedRows < BatchSize {
			logger.Info("Finished processing all users", zap.Int("total_processed", processedCount), zap.Int64("total_score_changes", scoreUpdatedCount), zap.Duration("duration", time.Since(startTime)))
			break
		}
	}

	return nil
}

type UpdateAggregatesJob struct {
	*BaseJob
}

func NewUpdateAggregatesJob(config config.Config, writePool database.DbPool, readPool *dbv1.DBPools) *UpdateAggregatesJob {
	runner := &UpdateAggregatesJobRunner{writePool: writePool, readPool: readPool}
	baseJob := NewBaseJob(BaseJobConfig{
		config:  config,
		jobName: "UpdateAggregatesJob",
		runner:  runner,
	})

	return &UpdateAggregatesJob{
		BaseJob: baseJob,
	}
}

func (j *UpdateAggregatesJob) ScheduleEvery(ctx context.Context, duration time.Duration) *UpdateAggregatesJob {
	j.BaseJob.ScheduleEvery(ctx, duration)
	return j
}
