package api

import (
	"errors"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// v1EventComments returns the top-level comment stream for a remix-contest event.
// Comments are authored by any signed-in user; a comment is considered a "post
// update" when its user_id matches the event's owner user_id (resolved client-side).
// Replies are not returned in this list — they come back nested inside the
// FullComment result just like track comments.
func (app *ApiServer) v1EventComments(c *fiber.Ctx) error {
	encodedEventID := c.Params("eventId")
	eventID, err := trashid.DecodeHashId(encodedEventID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid event id")
	}

	var eventRow struct {
		UserID    int32
		IsDeleted bool
	}
	err = app.pool.QueryRow(c.Context(), `
		SELECT user_id, COALESCE(is_deleted, false)
		FROM events
		WHERE event_id = $1
		LIMIT 1
	`, eventID).Scan(&eventRow.UserID, &eventRow.IsDeleted)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "event not found")
		}
		return err
	}
	if eventRow.IsDeleted {
		return fiber.NewError(fiber.StatusNotFound, "event not found")
	}

	var params GetCommentsParams
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myID := app.getMyId(c)

	// Pull top-level comment ids for this event, sorted and paginated the same
	// way track comments are. Threads are materialised below by FullComments.
	orderBy := `comments.created_at DESC`
	switch params.SortMethod {
	case "timestamp":
		orderBy = `comments.created_at ASC`
	case "top":
		orderBy = `(SELECT COUNT(*) FROM comment_reactions cr WHERE cr.comment_id = comments.comment_id) DESC, comments.created_at DESC`
	}

	sql := `
		SELECT comments.comment_id
		FROM comments
		LEFT JOIN comment_threads ct ON ct.comment_id = comments.comment_id
		WHERE comments.entity_type = 'Event'
		  AND comments.entity_id = @eventId
		  AND comments.is_delete = false
		  AND ct.parent_comment_id IS NULL
		ORDER BY ` + orderBy + `
		LIMIT @limit
		OFFSET @offset
	`

	args := pgx.NamedArgs{
		"eventId": eventID,
		"limit":   params.Limit,
		"offset":  params.Offset,
	}

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}
	commentIDs, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	comments, err := app.queries.FullComments(c.Context(), dbv1.GetCommentsParams{
		Ids:             commentIDs,
		MyID:            myID,
		IncludeUnlisted: true,
	})
	if err != nil {
		return err
	}

	// Collect related user ids so the UI can render avatars/handles in one shot.
	userIDs := []int32{eventRow.UserID}
	for _, co := range comments {
		userIDs = append(userIDs, int32(co.UserId))
		for _, m := range co.Mentions {
			userIDs = append(userIDs, int32(m.UserId))
		}
		for _, r := range co.Replies {
			userIDs = append(userIDs, int32(r.UserId))
		}
	}

	related, err := app.queries.Parallel(c.Context(), dbv1.ParallelParams{
		UserIds:      userIDs,
		TrackIds:     nil,
		MyID:         myID,
		AuthedWallet: app.tryGetAuthedWallet(c),
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": comments,
		"related": fiber.Map{
			"users":  related.UserList(),
			"tracks": related.TrackList(),
		},
		"event_user_id": trashid.MustEncodeHashID(int(eventRow.UserID)),
	})
}
