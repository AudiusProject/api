package api

import (
	"bridgerton.audius.co/queries"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1Users(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	ids := decodeIdList(c)

	if len(ids) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"status": 400,
			"error":  "id query param required",
		})
	}

	users, err := app.queries.FullUsers(c.Context(), queries.GetUsersParams{
		MyID: int32(myId),
		Ids:  ids,
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": users,
	})
}

func (app *ApiServer) v1UsersFollowers(c *fiber.Ctx) error {

	sql := `
	SELECT follower_user_id
	FROM follows
	JOIN users on follower_user_id = users.user_id
	JOIN aggregate_user using (user_id)
	WHERE followee_user_id = @userId
	  AND is_delete = false
	  AND is_deactivated = false
	ORDER BY follower_count desc
	LIMIT @limit
	OFFSET @offset
	`

	userId, err := trashid.DecodeHashId(c.Params("userId"))
	if err != nil {
		return err
	}

	users, err := app.queryFullUsers(c, sql, pgx.NamedArgs{
		"userId": userId,
	})

	return c.JSON(fiber.Map{
		"data": users,
	})
}

func (app *ApiServer) v1UsersFollowing(c *fiber.Ctx) error {

	sql := `
	SELECT followee_user_id
	FROM follows
	JOIN users on followee_user_id = users.user_id
	JOIN aggregate_user using (user_id)
	WHERE follower_user_id = @userId
	  AND is_delete = false
	  AND is_deactivated = false
	ORDER BY follower_count desc
	LIMIT @limit
	OFFSET @offset
	`

	userId, err := trashid.DecodeHashId(c.Params("userId"))
	if err != nil {
		return err
	}

	users, err := app.queryFullUsers(c, sql, pgx.NamedArgs{
		"userId": userId,
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": users,
	})
}

func (app *ApiServer) queryFullUsers(c *fiber.Ctx, sql string, args pgx.NamedArgs) ([]queries.FullUser, error) {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))

	args["limit"] = c.Query("limit", "20")
	args["offset"] = c.Query("offset", "0")

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return nil, err
	}

	userIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return nil, err
	}

	users, err := app.queries.FullUsers(c.Context(), queries.GetUsersParams{
		MyID: myId,
		Ids:  userIds,
	})
	if err != nil {
		return nil, err
	}

	userMap := map[int32]queries.FullUser{}
	for _, user := range users {
		userMap[user.UserID] = user
	}

	for idx, id := range userIds {
		users[idx] = userMap[id]
	}

	return users, nil
}
