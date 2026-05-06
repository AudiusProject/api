package api

import (
	"time"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// v1Me handles GET /v1/me
// Returns the authenticated user's profile. Supports any auth method that
// resolves to a user (OAuth PKCE token, wallet signature, JWT, or API key).
func (app *ApiServer) v1Me(c *fiber.Ctx) error {
	userId := app.getMyId(c)
	if userId == 0 {
		wallet := app.getAuthedWallet(c)
		id, err := app.getUserIDFromWallet(c.Context(), wallet)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "Could not resolve authenticated user")
		}
		userId = int32(id)
	}

	users, err := app.queries.Users(c.Context(), dbv1.GetUsersParams{
		Ids:  []int32{userId},
		MyID: userId,
	})
	if err != nil {
		app.logger.Error("Failed to query user for /me", zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to get user info")
	}
	if len(users) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}

	return c.JSON(fiber.Map{"data": users[0]})
}

// v1OAuthMe handles GET /v1/oauth/me
// Legacy endpoint that returns the user profile in the DecodedUserToken shape
// expected by @audius/sdk versions <= 14. Newer SDKs use /v1/me which returns
// the standard { data: User } envelope.
func (app *ApiServer) v1OAuthMe(c *fiber.Ctx) error {
	userId := app.getMyId(c)
	if userId == 0 {
		wallet := app.getAuthedWallet(c)
		id, err := app.getUserIDFromWallet(c.Context(), wallet)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "Could not resolve authenticated user")
		}
		userId = int32(id)
	}

	users, err := app.queries.Users(c.Context(), dbv1.GetUsersParams{
		Ids:  []int32{userId},
		MyID: userId,
	})
	if err != nil {
		app.logger.Error("Failed to query user for /oauth/me", zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to get user info")
	}
	if len(users) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "User not found")
	}

	user := users[0]
	response := fiber.Map{
		"userId":   user.ID,
		"name":     user.Name.String,
		"handle":   user.Handle.String,
		"verified": user.IsVerified,
		"sub":      user.ID,
		"iat":      time.Now().Unix(),
	}
	if user.ProfilePicture != nil {
		response["profilePicture"] = user.ProfilePicture
	}
	return c.JSON(response)
}
