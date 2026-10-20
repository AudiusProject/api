package api

import "github.com/gofiber/fiber/v2"

// Feed viewer IDs can be advisory (notably on For You). Only a verified
// caller may use the owner exception for an unreleased scheduled item.
func (app *ApiServer) feedScheduledOwnerID(c *fiber.Ctx) int32 {
	viewerID := app.getMyId(c)
	if viewerID != 0 && app.isAuthorizedRequest(c.Context(), viewerID, app.tryGetAuthedWallet(c)) {
		return viewerID
	}
	return 0
}
