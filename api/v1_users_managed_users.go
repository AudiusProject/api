package api

import (
	"strconv"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1UsersManagedUsers(c *fiber.Ctx) error {
	// Behavior of this field is a little odd. We only want to filter by it
	// if it is passed, but otherwise not use a default value for either.
	isApproved, err := getOptionalBool(c, "is_approved")
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid value for is_approved")
	}

	isRevoked, err := strconv.ParseBool(c.Query("is_revoked", "false"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid value for is_revoked")
	}
	params := dbv1.GetGrantsForGranteeUserIdParams{
		UserID:     app.getUserId(c),
		IsApproved: isApproved,
		IsRevoked:  isRevoked,
	}

	managedUsers, err := app.queries.FullManagedUsers(c.Context(), params)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": managedUsers,
	})
}
