package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) queryFullComments(c *fiber.Ctx, sql string, args pgx.NamedArgs) error {

	// sort
	switch c.Query("sort_method") {
	case "top":
		sql += ` ORDER BY (SELECT count(*) FROM comment_reactions WHERE comment_id = comments.comment_id) DESC, comments.created_at DESC `
	case "timestamp":
		sql += ` ORDER BY comments.created_at ASC `
	default:
		sql += ` ORDER BY comments.created_at DESC `
	}

	// pagination
	sql += `
	LIMIT @limit
	OFFSET @offset
	`

	args["limit"] = c.QueryInt("limit", 10)
	args["offset"] = c.QueryInt("offset", 0)

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	commentIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	comments, err := app.queries.FullComments(c.Context(), dbv1.GetCommentsParams{
		Ids:  commentIds,
		MyID: app.getMyId(c),
	})
	if err != nil {
		return err
	}

	// related
	userIds := []int32{}
	trackIds := []int32{}
	for _, c := range comments {
		userIds = append(userIds, int32(c.UserId))
		trackIds = append(trackIds, int32(c.EntityId))
		for _, m := range c.Mentions {
			userIds = append(userIds, int32(m.UserId))
		}
	}
	related, err := app.queries.Parallel(c.Context(), dbv1.ParallelParams{
		UserIds:  userIds,
		TrackIds: trackIds,
		MyID:     app.getMyId(c),
	})

	return c.JSON(fiber.Map{
		"my_id": app.getMyId(c),
		"data":  comments,
		"related": fiber.Map{
			"users":  related.UserList(),
			"tracks": related.TrackList(),
		},
	})
}
