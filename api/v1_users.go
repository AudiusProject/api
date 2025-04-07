package api

import (
	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// v1Users is a handler that retrieves full user data
func (app *ApiServer) v1Users(c *fiber.Ctx, minResponse bool) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	ids := decodeIdList(c)

	if len(ids) == 0 {
		return c.Status(400).JSON(fiber.Map{
			"status": 400,
			"error":  "id query param required",
		})
	}

	users, err := app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{
		MyID: int32(myId),
		Ids:  ids,
	})
	if err != nil {
		return err
	}

	// return users in same order as input list of ids
	// some ids may be not found...
	userMap := map[int32]dbv1.FullUser{}
	for _, user := range users {
		userMap[user.UserID] = user
	}

	orderedUsers := make([]dbv1.FullUser, 0, len(users))
	for _, id := range ids {
		if user, ok := userMap[id]; ok {
			orderedUsers = append(orderedUsers, user)
		}
	}

	if minResponse {
		return c.JSON(fiber.Map{
			"data": dbv1.ToMinUsers(orderedUsers),
		})
	}

	return c.JSON(fiber.Map{
		"data": orderedUsers,
	})
}

func (app *ApiServer) v1UsersFollowers(c *fiber.Ctx, minResponse bool) error {

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
	}, minResponse)
}

func (app *ApiServer) v1UsersFollowing(c *fiber.Ctx, minResponse bool) error {

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
	}, minResponse)
}

func (app *ApiServer) v1UsersMutuals(c *fiber.Ctx, minResponse bool) error {
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
	}, minResponse)
}

func (app *ApiServer) v1UsersSupporting(c *fiber.Ctx, minResponse bool) error {
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
	JOIN users ON a.receiver_user_id = user_id
	WHERE sender_user_id = @userId
	AND is_deactivated = false
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

	if minResponse {
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

func (app *ApiServer) queryFullUsers(c *fiber.Ctx, sql string, args pgx.NamedArgs, minResponse bool) error {
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

	if minResponse {
		return c.JSON(fiber.Map{
			"data": dbv1.ToMinUsers(users),
		})
	}

	return c.JSON(fiber.Map{
		"data": users,
	})
}
