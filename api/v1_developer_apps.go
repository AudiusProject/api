package api

import (
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

func (as *ApiServer) v1DeveloperApps(c *fiber.Ctx, minResponse bool) error {
	address := c.Params("address")
	if address == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing address parameter")
	}

	// Add the 0x prefix if it doesn't exist
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}

	developerApps, err := as.queries.FullDeveloperApps(c.Context(), dbv1.GetDeveloperAppsParams{
		Address: address,
	})
	if err != nil {
		return err
	}

	if len(developerApps) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "Developer app not found")
	}

	if minResponse {
		return c.JSON(fiber.Map{
			"data": dbv1.ToMinDeveloperApps(developerApps)[0],
		})
	}

	return c.JSON(fiber.Map{
		"data": developerApps[0],
	})
}
