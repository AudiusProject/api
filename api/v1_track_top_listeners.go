package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetTrackTopListenersParams struct {
	Limit  int `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

type UserWithPlayCount struct {
	User  dbv1.User `json:"user"`
	Count int64         `json:"count"`
}

type MinUserWithPlayCount struct {
	User  dbv1.User `json:"user"`
	Count int64     `json:"count"`
}

func (app *ApiServer) v1TrackTopListeners(c *fiber.Ctx) error {
	params := GetTrackTopListenersParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	sql := `
		SELECT c.user_id, c.play_count
		FROM (
			SELECT user_id,
				count(DISTINCT date_trunc('hour', created_at)) AS play_count
			FROM plays
			WHERE user_id IS NOT NULL
				AND play_item_id = @trackId
			GROUP BY user_id
		) c
		LEFT JOIN aggregate_user au USING (user_id)
		ORDER BY c.play_count DESC, au.follower_count DESC NULLS LAST, c.user_id ASC
		LIMIT @limit
		OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"trackId": c.Locals("trackId").(int),
		"limit":   params.Limit,
		"offset":  params.Offset,
	})
	if err != nil {
		return err
	}

	type UserPlayCountRow struct {
		UserID int32 `db:"user_id"`
		Count  int64 `db:"play_count"`
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByPos[UserPlayCountRow])
	if err != nil {
		return err
	}

	userIds := make([]int32, len(results))
	for i, result := range results {
		userIds[i] = result.UserID
	}

	users, err := app.queries.Users(c.Context(), dbv1.GetUsersParams{
		Ids:  userIds,
		MyID: app.getMyId(c),
	})
	if err != nil {
		return err
	}

	if app.getIsFull(c) {
		data := make([]UserWithPlayCount, len(users))
		for i, user := range users {
			data[i] = UserWithPlayCount{
				User:  user,
				Count: results[i].Count,
			}
		}
		return c.JSON(fiber.Map{
			"data": data,
		})
	} else {
		data := make([]MinUserWithPlayCount, len(users))
		for i, user := range users {
			data[i] = MinUserWithPlayCount{
				User:  user,
				Count: results[i].Count,
			}
		}
		return c.JSON(fiber.Map{
			"data": data,
		})
	}
}
