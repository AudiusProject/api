package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"bridgerton.audius.co/api/dbv1"
)

func (app *ApiServer) v1UsersTransactionsUsdc(c *fiber.Ctx) error {
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

	// TODO: Test transaction_type, add include_system_transactions, transaction_method filtering

	transactionTypeList := c.Query("type")

	var transactionTypes []string
	if transactionTypeList != "" {
		transactionTypes = strings.Split(transactionTypeList, ",")
	} else {
		transactionTypes = nil
	}

	transactions, err := app.queries.GetUserUsdcTransactions(c.Context(), dbv1.GetUserUsdcTransactionsParams{
		UserID:           app.getUserId(c),
		TransactionTypes: transactionTypes,
		SortMethod:       sortMethod,
		SortDirection:    sortDirection,
		LimitVal:         int32(limit),
		OffsetVal:        int32(offset),
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": transactions,
	})
}

func (app *ApiServer) v1UsersTransactionsUsdcCount(c *fiber.Ctx) error {
	// TODO: Add method, type, include_system_transactions filtering
	count, err := app.queries.GetUserUsdcTransactionsCount(c.Context(), dbv1.GetUserUsdcTransactionsCountParams{
		UserID: app.getUserId(c),
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": count,
	})
}
