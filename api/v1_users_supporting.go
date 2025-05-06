package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UsersSupporting(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	userId := app.getUserId(c)

	args := pgx.NamedArgs{
		"userId": userId,
	}

	args["limit"] = c.Query("limit", "20")
	args["offset"] = c.Query("offset", "0")

	type supportedUser struct {
		Rank           int           `json:"rank" db:"rank"`
		SenderUserID   int32         `json:"-" db:"sender_user_id"`
		ReceiverUserID int32         `json:"-" db:"receiver_user_id"`
		Amount         string        `json:"amount" db:"amount"`
		Receiver       dbv1.FullUser `json:"receiver" db:"-"`
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
	-- JOIN users ON a.receiver_user_id = user_id
	WHERE
		sender_user_id = @userId
		-- todo:
		-- these conditions should be here:
		-- but python will actually show deactivate / unavailable users
		-- so to minimize apidiff, skip the join above and do the wrong thing here
		-- AND is_deactivated = false
		-- AND is_available = true
	ORDER BY a.amount DESC, receiver_user_id ASC
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
		userIds = append(userIds, s.ReceiverUserID)
	}
	users, err := app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{
		MyID: myId,
		Ids:  userIds,
	})
	if err != nil {
		return err
	}

	userMap := map[int32]dbv1.FullUser{}
	for _, user := range users {
		userMap[user.UserID] = user
	}

	for idx, s := range supported {
		s.Receiver = userMap[s.ReceiverUserID]
		supported[idx] = s
	}

	if !app.getIsFull(c) {
		// Create a new array with MinUsers
		type minSupportedUser struct {
			supportedUser
			Receiver dbv1.MinUser `json:"receiver"`
		}

		minSupported := make([]minSupportedUser, len(supported))
		for i, user := range supported {
			minSupported[i] = minSupportedUser{
				supportedUser: user,
				Receiver:      dbv1.ToMinUser(user.Receiver),
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
