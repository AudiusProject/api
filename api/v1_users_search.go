package api

import (
	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/searcher"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1UsersSearch(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	limit := c.QueryInt("limit", 10)
	offset := c.QueryInt("offset", 0)

	q := searcher.UserSearchQuery{
		Query:      c.Query("query"),
		IsVerified: c.QueryBool("is_verified"),
	}

	dsl := searcher.BuildFunctionScoreDSL("follower_count", q.Map())
	userIds, err := searcher.SearchAndPluck(app.esClient, "users", dsl, limit, offset)
	if err != nil {
		return err
	}

	users, err := app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{
		Ids:  userIds,
		MyID: myId,
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": users,
	})
}
