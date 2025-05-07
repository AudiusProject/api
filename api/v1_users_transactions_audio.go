package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1UsersTransactionsAudio(c *fiber.Ctx) error {
	userID := app.getUserId(c)

	sortMethod := c.Query("sort_method", "date")
	sortDirection := c.Query("sort_direction", "desc")
	offsetVal := c.QueryInt("offset_val", 0)
	// TODO: make this a constant
	limitVal := c.QueryInt("limit_val", 100)

	// TODO: Validate sort method and direction
	transactions, err := app.queries.GetUserAudioTransactions(c.Context(), dbv1.GetUserAudioTransactionsParams{
		UserID:        userID,
		SortDirection: sortDirection,
		SortMethod:    sortMethod,
		OffsetVal:     int32(offsetVal),
		LimitVal:      int32(limitVal),
	})

	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": transactions,
	})
}

func (app *ApiServer) v1UsersTransactionsAudioCount(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"data": 0,
	})
}
