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

	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"userId": userId,
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

	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"userId": userId,
	})
}

func (app *ApiServer) v1UsersMutuals(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))

	sql := `
	SELECT x.follower_user_id
	FROM follows x
	JOIN aggregate_user au on x.follower_user_id = au.user_id
	JOIN follows me
	  ON me.follower_user_id = @myId
	 AND me.followee_user_id = x.follower_user_id
	 AND me.is_delete = false
	WHERE x.followee_user_id = @userId
	  AND x.is_delete = false
	ORDER BY follower_count DESC
	LIMIT @limit
	OFFSET @offset
	`

	userId, err := trashid.DecodeHashId(c.Params("userId"))
	if err != nil {
		return err
	}

	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"myId":   myId,
		"userId": userId,
	})
}

func (app *ApiServer) v1UsersSupporting(c *fiber.Ctx) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))

	userId, err := trashid.DecodeHashId(c.Params("userId"))
	if err != nil {
		return err
	}

	args := pgx.NamedArgs{
		"userId": userId,
	}

	args["limit"] = c.Query("limit", "20")
	args["offset"] = c.Query("offset", "0")

	type supportedUser struct {
		Rank           int              `json:"rank" db:"rank"`
		SenderUserID   int32            `json:"-" db:"sender_user_id"`
		ReceiverUserID int32            `json:"-" db:"receiver_user_id"`
		Amount         string           `json:"amount" db:"amount"`
		Receiver       queries.FullUser `json:"receiver" db:"-"`
	}

	sql := `
	WITH ranked_donations AS (
		SELECT
			sender_user_id,
			receiver_user_id,
			MAX(amount) AS amount,
			RANK() OVER (
				PARTITION BY receiver_user_id
				ORDER BY MAX(amount) DESC
			) AS rank
		FROM aggregate_user_tips
		JOIN users ON receiver_user_id = user_id
		WHERE is_deactivated = false
		GROUP BY receiver_user_id, sender_user_id
	)
	SELECT
		sender_user_id,
		receiver_user_id,
		ranked_donations.amount || '0000000000' as amount,
		rank
	FROM ranked_donations
	WHERE sender_user_id = @userId
	ORDER BY ranked_donations.amount DESC, receiver_user_id ASC
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
	users, err := app.queries.FullUsers(c.Context(), queries.GetUsersParams{
		MyID: myId,
		Ids:  userIds,
	})
	if err != nil {
		return err
	}

	userMap := map[int32]queries.FullUser{}
	for _, user := range users {
		userMap[user.UserID] = user
	}

	for idx, s := range supported {
		s.Receiver = userMap[s.ReceiverUserID]
		supported[idx] = s
	}

	return c.JSON(fiber.Map{
		"data": supported,
	})

}

func (app *ApiServer) queryFullUsers(c *fiber.Ctx, sql string, args pgx.NamedArgs) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))

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

	users, err := app.queries.FullUsers(c.Context(), queries.GetUsersParams{
		MyID: myId,
		Ids:  userIds,
	})
	if err != nil {
		return err
	}

	userMap := map[int32]queries.FullUser{}
	for _, user := range users {
		userMap[user.UserID] = user
	}

	for idx, id := range userIds {
		users[idx] = userMap[id]
	}

	return c.JSON(fiber.Map{
		"data": users,
	})
}
