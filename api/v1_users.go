package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// v1Users is a handler that retrieves full user data
func (app *ApiServer) v1Users(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	ids := decodeIdList(c)

	if len(ids) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "no user ids provided")
	}

	users, err := app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{
		MyID: myId,
		Ids:  ids,
	})
	if err != nil {
		return err
	}

	return v1UsersResponse(c, users)
}

type GetUsersParams struct {
	Limit  int `query:"limit" default:"20" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

// a generic responder for all the simple user lists:
// followers, followees, reposters, savers, etc.
func (app *ApiServer) queryFullUsers(c *fiber.Ctx, sql string, args pgx.NamedArgs) error {
	myId := app.getMyId(c)

	params := GetUsersParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	args["limit"] = params.Limit
	args["offset"] = params.Offset

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

	return v1UsersResponse(c, users)
}
