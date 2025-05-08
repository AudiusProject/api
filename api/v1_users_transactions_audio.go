package api

import (
	"github.com/gofiber/fiber/v2"

	"bridgerton.audius.co/api/dbv1"
)

func (app *ApiServer) v1UsersTransactionsAudio(c *fiber.Ctx) error {
	sortMethod := c.Query("sort_method", "date")
	if sortMethod != "date" && sortMethod != "type" {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid sort method")
	}
	sortDirection := c.Query("sort_direction", "desc")
	if sortDirection != "asc" && sortDirection != "desc" {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid sort direction")
	}

	limit := c.QueryInt("limit", 100)
	offset := c.QueryInt("offset", 0)

	transactionTypes := c.Context().QueryArgs().PeekMulti("type")
	if len(transactionTypes) == 0 {
		transactionTypes = nil
	}

	transactions, err := app.queries.GetUserAudioTransactions(c.Context(), dbv1.GetUserAudioTransactionsParams{
		UserID:        app.getUserId(c),
		SortMethod:    sortMethod,
		SortDirection: sortDirection,
		LimitVal:      int32(limit),
		OffsetVal:     int32(offset),
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": transactions,
	})
}

func (app *ApiServer) v1UsersTransactionsAudioCount(c *fiber.Ctx) error {
	count, err := app.queries.GetUserAudioTransactionsCount(c.Context(), app.getUserId(c))
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": count,
	})
}
