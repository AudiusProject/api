package api

import (
	"log"
	"strconv"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

func (app *ApiServer) v1UsersManagers(c *fiber.Ctx) error {
	// Behavior of this field is a little odd. We only want to filter by it
	// if it is passed, but otherwise not use a default value for either.
	var isApproved *bool
	if approvedStr := c.Query("is_approved"); approvedStr != "" {
		parsed, err := strconv.ParseBool(approvedStr)
		if err != nil {
			return fiber.NewError(fiber.StatusBadRequest, "Invalid value for is_approved")
		}
		isApproved = &parsed
	}

	isRevoked := c.QueryBool("is_revoked", false)
	log.Printf("isApproved: %v, isRevoked: %v", isApproved, isRevoked)
	params := dbv1.GetGrantsForUserIdParams{
		UserID:     int32(c.Locals("userId").(int)),
		IsApproved: pgtype.Bool{Bool: isApproved != nil && *isApproved, Valid: isApproved != nil},
		IsRevoked:  isRevoked,
	}

	managers, err := app.queries.FullManagers(c.Context(), params)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": managers,
	})
}
