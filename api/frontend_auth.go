package api

import (
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// requireFrontendAppAuth returns a middleware that validates Bearer token and checks
// that the given frontend app (identified by its private key secret) has a grant from the user.
// Must run after requireUserIdMiddleware.
func (app *ApiServer) requireFrontendAppAuth(secret string, appName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if secret == "" {
			app.logger.Error(appName+" secret not configured", zap.String("app", appName))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": appName + " not configured",
			})
		}

		authHeader := c.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing or invalid Authorization header. Use Bearer <oauth_token>",
			})
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		pathUserId := app.getUserId(c)
		if pathUserId == 0 {
			return fiber.NewError(fiber.StatusBadRequest, "invalid userId")
		}

		jwtUserId, err := app.validateOAuthJWTTokenToUserId(c.Context(), token)
		if err != nil {
			return err
		}

		if int32(jwtUserId) != pathUserId {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "Token userId does not match path userId",
			})
		}

		// Derive app address from private key
		privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(secret, "0x"))
		if err != nil {
			app.logger.Error("Invalid "+appName+" secret", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": appName + " misconfigured",
			})
		}
		appAddress := strings.ToLower(crypto.PubkeyToAddress(privateKey.PublicKey).Hex())

		if !app.isAuthorizedRequest(c.Context(), pathUserId, appAddress) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "User has not granted the " + strings.ToLower(appName) + " write access. Log in with OAuth scope 'write'.",
			})
		}

		return c.Next()
	}
}

// requireDeveloperAppAuth validates OAuth Bearer token and sets userId from JWT.
// Used for /developer-apps routes where userId is not in the path.
// Requires the Plans app to have a grant from the user.
func (app *ApiServer) requireDeveloperAppAuth(secret string, appName string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if secret == "" {
			app.logger.Error(appName+" secret not configured", zap.String("app", appName))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": appName + " not configured",
			})
		}

		authHeader := c.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"error": "Missing or invalid Authorization header. Use Bearer <oauth_token>",
			})
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		jwtUserId, err := app.validateOAuthJWTTokenToUserId(c.Context(), token)
		if err != nil {
			return err
		}
		userId := int32(jwtUserId)
		c.Locals("userId", int(userId))

		privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(secret, "0x"))
		if err != nil {
			app.logger.Error("Invalid "+appName+" secret", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": appName + " misconfigured",
			})
		}
		appAddress := strings.ToLower(crypto.PubkeyToAddress(privateKey.PublicKey).Hex())

		if !app.isAuthorizedRequest(c.Context(), userId, appAddress) {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "User has not granted the " + strings.ToLower(appName) + " write access. Log in with OAuth scope 'write'.",
			})
		}

		return c.Next()
	}
}
