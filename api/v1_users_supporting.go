package api

import (
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersSupportingQueryParams struct {
	Limit  int `query:"limit" default:"20" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}
type GetUsersSupportingRouteParams struct {
	SupportedUserId trashid.HashId `param:"supportedUserId"`
}

type SupportedUser struct {
	Rank           int           `json:"rank" db:"rank"`
	SenderUserID   int32         `json:"-" db:"sender_user_id"`
	ReceiverUserID int32         `json:"-" db:"receiver_user_id"`
	Amount         string        `json:"amount" db:"amount"`
	Receiver       dbv1.FullUser `json:"receiver" db:"-"`
}
type MinSupportedUser struct {
	SupportedUser
	Receiver dbv1.MinUser `json:"receiver"`
}

func (app *ApiServer) v1UsersSupporting(c *fiber.Ctx) error {
	query := GetUsersSupportingQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &query); err != nil {
		return err
	}
	params := GetUsersSupportingRouteParams{}
	if err := c.ParamsParser(&params); err != nil {
		return err
	}
	myId := app.getMyId(c)
	userId := app.getUserId(c)

	args := pgx.NamedArgs{
		"userId":      userId,
		"limit":       query.Limit,
		"offset":      query.Offset,
		"supporterId": params.SupportedUserId,
	}

	filters := []string{
		"sender_user_id = @userId",
		// TODO: Enable these. Currently disabled because python doesn't do the right thing.
		// "users.is_deactivated = FALSE",
		// "users.is_available = TRUE",
	}
	if params.SupportedUserId != 0 {
		filters = append(filters, "receiver_user_id = @supporterId")
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
		` + strings.Join(filters, " AND ") + `
	ORDER BY a.amount DESC, receiver_user_id ASC
	LIMIT @limit
	OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	supported, err := pgx.CollectRows(rows, pgx.RowToStructByName[SupportedUser])
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
		minSupported := make([]MinSupportedUser, len(supported))
		for i, user := range supported {
			minSupported[i] = MinSupportedUser{
				SupportedUser: user,
				Receiver:      dbv1.ToMinUser(user.Receiver),
			}
		}

		if params.SupportedUserId != 0 {
			return c.JSON(fiber.Map{
				"data": minSupported[0],
			})
		}

		return c.JSON(fiber.Map{
			"data": minSupported,
		})
	}

	if params.SupportedUserId != 0 {
		return c.JSON(fiber.Map{
			"data": supported[0],
		})
	}

	return c.JSON(fiber.Map{
		"data": supported,
	})
}
