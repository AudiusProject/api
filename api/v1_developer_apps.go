package api

import (
	"strings"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

func (as *ApiServer) v1DeveloperAppByAddress(c *fiber.Ctx) error {
	address := c.Params("address")
	if address == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing address parameter")
	}

	// Add the 0x prefix if it doesn't exist
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}

	developerApp, err := as.queries.GetDeveloperAppByAddress(c.Context(), address)
	if err != nil {
		return err
	}

	// Encode the user_id as a trashid
	userId, _ := trashid.EncodeHashId(int(*developerApp.UserID))

	// Create a formatted response with encoded user_id
	formattedApp := fiber.Map{
		"address":     developerApp.Address,
		"user_id":     userId,
		"name":        developerApp.Name,
		"description": developerApp.Description,
		"image_url":   developerApp.ImageUrl,
	}

	return c.JSON(fiber.Map{
		"data": formattedApp,
	})
}
