package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v5"
)

// this handler currently serves both tracks + user comment routes
// but should probably split apart and use "list of ids" pattern
// and follow the app.queries.FullComments pattern
// which is needed to attach replies

// TODO: need to attach replies + reply_count
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

	type Comment struct {
		Id         trashid.HashId `json:"id"`
		EntityType string         `json:"entity_type"`
		EntityId   trashid.HashId `json:"entity_id"`
		UserId     trashid.HashId `json:"user_id"`
		Message    string         `json:"message"`
		Mentions   []struct {
			UserId int    `json:"user_id"`
			Handle string `json:"handle"`
		} `json:"mentions"`
		TrackTimestampS      pgtype.Int4 `json:"track_timestamp_s"`
		IsMuted              bool        `json:"is_muted"`
		IsEdited             bool        `json:"is_edited"`
		IsCurrentUserReacted bool        `json:"is_current_user_reacted"`
		IsArtistReacted      bool        `json:"is_artist_reacted"`
		IsTombstone          bool        `json:"is_tombstone"`
		ReactCount           int         `json:"react_count"`
		CreatedAt            time.Time   `json:"created_at"`
		UpdatedAt            time.Time   `json:"updated_at"`
	}

	sql := `
	SELECT
		comment_id as id,
		entity_type,
		entity_id,
		user_id,
		text as message,

		(
			SELECT json_agg(
				json_build_object(
					'user_id', m.user_id,
					'handle', handle
				)
			)
			FROM (
				SELECT user_id, handle FROM comment_mentions
				JOIN users USING (user_id)
				WHERE comment_id = comments.comment_id
			) m
		)::jsonb as mentions,

		track_timestamp_s,

		(
			SELECT count(*)
			FROM comment_reactions
			WHERE comment_id = comments.comment_id
			AND is_delete = false
		) as react_count,


		-- reply_count

		is_edited,

		EXISTS (
			SELECT 1
			FROM comment_reactions
			WHERE comment_id = comments.comment_id
			AND user_id = @my_id
			AND is_delete = false
		) AS is_current_user_reacted,

		EXISTS (
			SELECT 1
			FROM comment_reactions
			WHERE comment_id = comments.comment_id
			AND user_id = tracks.owner_id
			AND is_delete = false
		) AS is_artist_reacted,

		-- is_tombstone

		coalesce((
			SELECT is_muted
			FROM comment_notification_settings mutes
			WHERE @my_id > 0
			AND mutes.user_id = @my_id
			AND mutes.entity_type = entity_type
			AND mutes.entity_id = entity_id
			LIMIT 1
		), false) as is_muted,

		comments.created_at,
		comments.updated_at

		-- replies
	FROM comments
	JOIN tracks ON entity_id = track_id
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

	comments, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[Comment])
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
		"data": comments,
		"related": fiber.Map{
			"users":  related.UserList(),
			"tracks": related.TrackList(),
		},
	})
}
