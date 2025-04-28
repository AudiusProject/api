package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// this handler currently serves both tracks + user comment routes
// but should probably split apart and use "list of ids" pattern
// and follow the app.queries.FullComments pattern
// which is needed to attach replies

// TODO: properly do tombstone field
func (app *ApiServer) v1Comments(c *fiber.Ctx) error {

	whereClause := ``
	args := pgx.NamedArgs{
		"user_id": c.Locals("userId"),
		"my_id":   app.getMyId(c),
		"limit":   c.QueryInt("limit", 10),
		"offset":  c.QueryInt("offset", 0),
	}
	if c.Locals("trackId") != nil {
		whereClause = "parent_comment_id IS NULL AND entity_id = @track_id"
		args["track_id"] = c.Locals("trackId")
	} else if c.Locals("userId") != nil {
		whereClause = "user_id = @user_id"
		args["user_id"] = c.Locals("userId")
	} else {
		return fiber.NewError(400, "userId or trackId is required")
	}

	sql := `
	SELECT comment_id as id
	FROM comments
	LEFT JOIN comment_threads USING (comment_id)
	WHERE ` + whereClause + `
	AND entity_type = 'Track'
	AND comments.is_delete = false
	ORDER BY comments.created_at DESC
	LIMIT @limit
	OFFSET @offset
	`

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
	// todo: add CommentIds to Parallel
	// todo: do this in FullComments with a parallel call
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
		"data": comments,
		"related": fiber.Map{
			"users":  related.UserList(),
			"tracks": related.TrackList(),
		},
	})
}
