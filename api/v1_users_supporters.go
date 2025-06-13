package api

import (
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersSupportersQueryParams struct {
	Limit  int `query:"limit" default:"20" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}
type GetUsersSupportersRouteParams struct {
	SupporterId trashid.HashId `param:"supporterId"`
}

type Supporter struct {
	Rank           int           `json:"rank" db:"rank"`
	SenderUserID   int32         `json:"-" db:"sender_user_id"`
	ReceiverUserID int32         `json:"-" db:"receiver_user_id"`
	Amount         string        `json:"amount" db:"amount"`
	Sender         dbv1.FullUser `json:"sender" db:"-"`
}

type MinSupporter struct {
	Supporter
	Sender dbv1.MinUser `json:"sender"`
}

func (app *ApiServer) v1UsersSupporters(c *fiber.Ctx) error {
	query := GetUsersSupportersQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &query); err != nil {
		return err
	}
	params := GetUsersSupportersRouteParams{}
	if err := c.ParamsParser(&params); err != nil {
		return err
	}

	myId := app.getMyId(c)
	userId := app.getUserId(c)

	args := pgx.NamedArgs{
		"userId":      userId,
		"limit":       query.Limit,
		"offset":      query.Offset,
		"supporterId": params.SupporterId,
	}

	filters := []string{"receiver_user_id = @userId"}
	if params.SupporterId != 0 {
		filters = append(filters, "sender_user_id = @supporterId")
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
		` + strings.Join(filters, " AND ") + `
	ORDER BY a.amount DESC, sender_user_id ASC
	LIMIT @limit
	OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	supporters, err := pgx.CollectRows(rows, pgx.RowToStructByName[Supporter])
	if err != nil {
		return err
	}

	userIds := []int32{}
	for _, s := range supporters {
		userIds = append(userIds, s.SenderUserID)
	}
	userMap, err := app.queries.FullUsersKeyed(c.Context(), dbv1.GetUsersParams{
		MyID: myId,
		Ids:  userIds,
	})
	if err != nil {
		return err
	}

	for idx, s := range supporters {
		s.Sender = userMap[s.SenderUserID]
		supporters[idx] = s
	}

	if !app.getIsFull(c) {
		// Create a new array with MinUsers
		minSupporters := make([]MinSupporter, len(supporters))
		for i, user := range supporters {
			minSupporters[i] = MinSupporter{
				Supporter: user,
				Sender:    dbv1.ToMinUser(user.Sender),
			}
		}

		if params.SupporterId != 0 {
			return c.JSON(fiber.Map{
				"data": minSupporters[0],
			})
		}

		return c.JSON(fiber.Map{
			"data": minSupporters,
		})
	}

	if params.SupporterId != 0 {
		return c.JSON(fiber.Map{
			"data": supporters[0],
		})
	}

	return c.JSON(fiber.Map{
		"data": supporters,
	})
}
