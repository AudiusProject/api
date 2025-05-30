package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersSupportersParams struct {
	Limit  int `query:"limit" default:"20" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

func (app *ApiServer) v1UsersSupporters(c *fiber.Ctx) error {
	params := GetUsersSupportersParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}
	myId := app.getMyId(c)
	userId := app.getUserId(c)

	args := pgx.NamedArgs{
		"userId": userId,
		"limit":  params.Limit,
		"offset": params.Offset,
	}

	type supportedUser struct {
		Rank           int           `json:"rank" db:"rank"`
		SenderUserID   int32         `json:"-" db:"sender_user_id"`
		ReceiverUserID int32         `json:"-" db:"receiver_user_id"`
		Amount         string        `json:"amount" db:"amount"`
		Sender         dbv1.FullUser `json:"sender" db:"-"`
	}

	sql := `
	SELECT
		sender_user_id,
		receiver_user_id,
		amount || '0000000000' as amount,
		(
			SELECT count(*) + 1
			FROM aggregate_user_tips b
			WHERE b.receiver_user_id = a.receiver_user_id
			  AND b.amount > a.amount
		) as rank
	FROM aggregate_user_tips a
	-- JOIN users ON a.sender_user_id = user_id
	WHERE
		receiver_user_id = @userId
		-- todo:
		-- do the wrong thing here to match python reponse
		-- (see comment in v1_users_supporting.go)
		-- AND is_deactivated = false
		-- AND is_available = true
	ORDER BY a.amount DESC, sender_user_id ASC
	LIMIT @limit
	OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	supported, err := pgx.CollectRows(rows, pgx.RowToStructByName[supportedUser])
	if err != nil {
		return err
	}

	userIds := []int32{}
	for _, s := range supported {
		userIds = append(userIds, s.SenderUserID)
	}
	userMap, err := app.queries.FullUsersKeyed(c.Context(), dbv1.GetUsersParams{
		MyID: myId,
		Ids:  userIds,
	})
	if err != nil {
		return err
	}

	for idx, s := range supported {
		s.Sender = userMap[s.SenderUserID]
		supported[idx] = s
	}

	if !app.getIsFull(c) {
		// Create a new array with MinUsers
		type minSupportedUser struct {
			supportedUser
			Sender dbv1.MinUser `json:"sender"`
		}

		minSupported := make([]minSupportedUser, len(supported))
		for i, user := range supported {
			minSupported[i] = minSupportedUser{
				supportedUser: user,
				Sender:        dbv1.ToMinUser(user.Sender),
			}
		}

		return c.JSON(fiber.Map{
			"data": minSupported,
		})
	}

	return c.JSON(fiber.Map{
		"data": supported,
	})
}
