package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetTrackTopListenersParams struct {
	Limit  int `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

type FullUserWithPlayCount struct {
	User  dbv1.FullUser `json:"user"`
	Count int64         `json:"count"`
}

type MinUserWithPlayCount struct {
	User  dbv1.MinUser `json:"user"`
	Count int64        `json:"count"`
}

func (app *ApiServer) v1TrackTopListeners(c *fiber.Ctx) error {
	params := GetTrackTopListenersParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	sql := `
		WITH deduped AS (
			SELECT DISTINCT play_item_id, user_id, date_trunc('hour', created_at) as created_at
			FROM plays
			WHERE user_id IS NOT NULL
				AND play_item_id = @trackId
			),
			counted as (
				SELECT user_id, count(*) as play_count
				FROM deduped
				GROUP BY 1
			)
		SELECT counted.user_id, counted.play_count
		FROM counted
		LEFT JOIN aggregate_user USING (user_id)
		ORDER BY play_count DESC, follower_count DESC, counted.user_id ASC
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

	type UserWithPlayCount struct {
		UserID int32 `db:"user_id"`
		Count  int64 `db:"play_count"`
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByPos[UserWithPlayCount])
	if err != nil {
		return err
	}

	userIds := make([]int32, len(results))
	for i, result := range results {
		userIds[i] = result.UserID
	}

	users, err := app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{
		Ids:  userIds,
		MyID: app.getMyId(c),
	})
	if err != nil {
		return err
	}

	if c.Locals("isFull").(bool) {
		data := make([]FullUserWithPlayCount, len(users))
		for i, user := range users {
			data[i] = FullUserWithPlayCount{
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
				User:  dbv1.ToMinUser(user),
				Count: results[i].Count,
			}
		}
		return c.JSON(fiber.Map{
			"data": data,
		})
	}
}
