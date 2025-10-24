package jobs

import (
	"context"
	"strings"
	"time"

	dbv1 "api.audius.co/api/dbv1"
	"api.audius.co/database"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type UpdateAggregatesJob struct {
	writePool database.DbPool
	readPool  *dbv1.DBPools
	logger    *zap.Logger
}

const (
	UpdateUserScoresBatchSize = 5000
)

type UpdateAggregatesJobConfig struct {
	WritePool database.DbPool
	ReadPool  *dbv1.DBPools
	Logger    *zap.Logger
}

func NewUpdateAggregatesJob(config UpdateAggregatesJobConfig) *UpdateAggregatesJob {
	return &UpdateAggregatesJob{
		writePool: config.WritePool,
		readPool:  config.ReadPool,
		logger:    config.Logger,
	}
}

func (j *UpdateAggregatesJob) Run(ctx context.Context) error {
	if err := j.updateScores(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
		return err
	}
	return nil
}

func (j *UpdateAggregatesJob) updateScores(ctx context.Context) error {
	startTime := time.Now()
	j.logger.Info("Starting user score update job")

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
			j.logger.Info("Processing batch", zap.String("lastCreatedAt", lastCreatedAt.Format(time.RFC3339)), zap.Int32("lastUserID", *lastUserID))
		} else {
			j.logger.Info("Processing first batch")
		}

		query := `
			WITH batch AS (
				SELECT u.user_id, u.created_at
				FROM users u
				WHERE ` + strings.Join(filters, " AND ") + `
				ORDER BY u.created_at DESC, u.user_id DESC
				LIMIT @batchSize
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
			features AS (
				SELECT
					u.user_id,
					b.created_at,
					COALESCE((
						SELECT udph.hours_with_play
						FROM user_distinct_play_hours udph
						WHERE udph.user_id = b.user_id
					), 0) AS play_count,
					COALESCE((
						SELECT t.track_count
						FROM user_distinct_play_tracks t
						WHERE t.user_id = b.user_id
					),0)::bigint AS distinct_tracks_played,
					COALESCE(usf.challenge_count, 0)::bigint        AS challenge_count,
					COALESCE(au.following_count, 0)::bigint       AS following_count,
					COALESCE(au.follower_count,  0)::bigint       AS follower_count,
					COALESCE(cb.block_count,     0)::bigint       AS chat_block_count,
					( ((u.handle_lc ILIKE '%audius%') OR (lower(u.name) ILIKE '%audius%'))
					AND u.is_verified = FALSE )                 AS is_audius_impersonator,
					( u.is_verified = FALSE
					AND (u.handle_lc ILIKE '%airdrop%' OR lower(u.name) LIKE '%airdrop%') )
																AS has_badwords,
					CASE
					WHEN COALESCE(au.follower_count, 0) > 1000 THEN 100
					WHEN COALESCE(au.follower_count, 0) = 0     THEN 0
					ELSE COALESCE(k.karma_sum, 0)
					END::bigint                                   AS karma
				FROM batch b
				JOIN users u                    ON u.user_id = b.user_id
				LEFT JOIN user_score_features usf ON usf.user_id = b.user_id
				LEFT JOIN chat_blocks cb        ON cb.user_id = b.user_id
				LEFT JOIN aggregate_user au     ON au.user_id = b.user_id
				LEFT JOIN followers_karma k     ON k.user_id  = b.user_id
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
		res, err := j.readPool.Query(ctx, query, pgx.NamedArgs{
			"batchSize":    UpdateUserScoresBatchSize,
			"cursorTime":   lastCreatedAt,
			"cursorUserId": lastUserID,
		})
		if err != nil {
			j.logger.Error("Failed to execute batch read query", zap.Error(err))
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
		tag, err := j.writePool.Exec(ctx, `
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
			j.logger.Error("Failed to execute update query", zap.Error(err))
			return err
		}

		processedCount += fetchedRows
		scoreUpdatedCount += tag.RowsAffected()
		j.logger.Info("Processed batch",
			zap.Int("batch_size", fetchedRows),
			zap.Int32("last_user_id", userID),
			zap.String("last_created_at", lastCreatedAt.Format(time.RFC3339)),
			zap.Int("total_processed", processedCount),
			zap.Int64("total_scores_changes", tag.RowsAffected()),
			zap.Duration("read_query_duration", readQueryDuration),
			zap.Duration("write_query_duration", writeQueryDuration))

		if fetchedRows < UpdateUserScoresBatchSize {
			j.logger.Info("Finished processing all users",
				zap.Int("total_processed", processedCount),
				zap.Int64("total_score_changes", scoreUpdatedCount),
				zap.Duration("duration", time.Since(startTime)))
			break
		}
	}

	return nil
}
