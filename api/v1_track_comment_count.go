package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

const karmaCommentCountThreshold = 1700000

func (app *ApiServer) v1TrackCommentCount(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	trackId := c.Locals("trackId")

	sql := `
	WITH 
	track AS (
		SELECT track_id, owner_id
		FROM tracks
		WHERE track_id = @trackId
	),

	-- Users muted by high-karma users
	muted_by_karma AS (
		SELECT muted_user_id
		FROM muted_users
		JOIN aggregate_user ON muted_users.user_id = aggregate_user.user_id
		WHERE muted_users.is_delete = false
		GROUP BY muted_user_id
		HAVING SUM(aggregate_user.follower_count) >= @karmaCommentCountThreshold
	),

	-- Comments reported by high-karma users
	high_karma_reporters AS (
		SELECT comment_id
		FROM comment_reports
		JOIN aggregate_user ON comment_reports.user_id = aggregate_user.user_id
		WHERE comment_reports.is_delete = false
		GROUP BY comment_id
		HAVING SUM(aggregate_user.follower_count) >= @karmaCommentCountThreshold
	)

	SELECT COUNT(*) as comment_count
	FROM comments
	LEFT JOIN track ON comments.entity_id = track.track_id
	LEFT JOIN comment_reports ON comments.comment_id = comment_reports.comment_id
	LEFT JOIN muted_users ON (
		muted_users.muted_user_id = comments.user_id
		AND (
			muted_users.user_id = @myId
			OR muted_users.user_id = track.owner_id
			OR muted_users.muted_user_id IN (SELECT muted_user_id FROM muted_by_karma)
		)
		AND @myId != comments.user_id  -- always show comments to their poster
	)
	WHERE comments.entity_id = @trackId
		AND comments.entity_type = 'Track'
		AND comments.is_delete = false
		-- Filter out comments that are reported, unless:
		-- 1. No report exists, OR
		-- 2. Report is not from current user or track owner AND comment is not reported by high-karma users, OR  
		-- 3. Report is deleted
		AND (
			comment_reports.comment_id IS NULL
			OR (
				comment_reports.user_id != COALESCE(@myId, 0)
				AND comment_reports.user_id != track.owner_id
				AND comments.comment_id NOT IN (SELECT comment_id FROM high_karma_reporters)
			)
			OR comment_reports.is_delete = true
		)
		-- Filter out muted comments unless the mute relationship is deleted
		AND (
			muted_users.muted_user_id IS NULL
			OR muted_users.is_delete = true
		)
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"myId":                       myId,
		"trackId":                    trackId,
		"karmaCommentCountThreshold": karmaCommentCountThreshold,
	})
	if err != nil {
		return err
	}

	count, err := pgx.CollectOneRow(rows, pgx.RowTo[int])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": count,
	})
}
