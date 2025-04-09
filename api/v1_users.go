package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// v1Users is a handler that retrieves full user data
func (app *ApiServer) v1Users(c *fiber.Ctx) error {
	myId := c.Locals("myId").(int)
	ids := decodeIdList(c)

	if len(ids) == 0 {
		return sendError(c, 400, "id query param required")
	}

	users, err := app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{
		MyID: int32(myId),
		Ids:  ids,
	})
	if err != nil {
		return err
	}

	return v1UserResponse(c, users)
}

// a generic responder for all the simple user lists:
// followers, followees, reposters, savers, etc.
func (app *ApiServer) queryFullUsers(c *fiber.Ctx, sql string, args pgx.NamedArgs) error {
	myId := c.Locals("myId")

	args["limit"] = c.Query("limit", "20")
	args["offset"] = c.Query("offset", "0")

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	userIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
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

	for idx, id := range userIds {
		users[idx] = userMap[id]
	}

	return v1UserResponse(c, users)
}
