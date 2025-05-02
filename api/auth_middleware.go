package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
)

// Recover user id and wallet from signature headers
func (app *ApiServer) recoverAuthorityFromSignatureHeaders(c *fiber.Ctx) (int32, string) {
	message := c.Get("Encoded-Data-Message")
	signature := c.Get("Encoded-Data-Signature")
	if message == "" || signature == "" {
		return 0, ""
	}

	encodedToRecover := []byte(message)
	prefixedMessage := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(encodedToRecover), encodedToRecover))
	finalHash := crypto.Keccak256Hash(prefixedMessage)
	signatureBytes := common.FromHex(signature)
	if signatureBytes[64] >= 27 {
		signatureBytes[64] -= 27
	}

	publicKey, err := crypto.SigToPub(finalHash.Bytes(), signatureBytes)
	if err != nil {
		return 0, ""
	}

	recoveredAddr := crypto.PubkeyToAddress(*publicKey)
	walletLower := strings.ToLower(recoveredAddr.Hex())

	// check cache
	if hit, ok := app.resolveWalletCache.Get(walletLower); ok {
		return hit, walletLower
	}

	var userId int32
	err = app.pool.QueryRow(
		c.Context(),
		`
		SELECT user_id FROM users
		WHERE
			wallet = $1
			AND is_current = true
		ORDER BY handle_lc IS NOT NULL, created_at ASC
		LIMIT 1
		`,
		walletLower,
	).Scan(&userId)

	if err != nil {
		return 0, walletLower
	}

	app.resolveWalletCache.Set(walletLower, userId)

	return userId, walletLower
}

// Checks if authedWallet is authorized to act on behalf of userId
func (app *ApiServer) isAuthorizedRequest(ctx context.Context, userId int32, authedWallet string) bool {
	cacheKey := fmt.Sprintf("%d:%s", userId, authedWallet)
	if hit, ok := app.resolveGrantCache.Get(cacheKey); ok {
		return hit
	}

	var isAuthorized bool
	err := app.pool.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM grants
				WHERE
					is_current = true
					AND user_id = $1
					AND grantee_address = $2
					AND is_approved = true
					AND is_revoked = false
			)
		`, userId, authedWallet).Scan(&isAuthorized)

	if err != nil {
		return false
	}

	app.resolveGrantCache.Set(cacheKey, isAuthorized)
	return isAuthorized
}

func (app *ApiServer) getAuthedUserId(c *fiber.Ctx) int32 {
	return int32(c.Locals("authedUserId").(int32))
}

func (app *ApiServer) getAuthedWallet(c *fiber.Ctx) string {
	return c.Locals("authedWallet").(string)
}

// Middleware to set authedUserId and authedWallet in context
// Returns a 403 if either
// - the user is not authorized to act on behalf of "myId"
// - the user is not authorized to act on behalf of "requestedWallet"
func (app *ApiServer) authMiddleware(c *fiber.Ctx) error {
	userId, wallet := app.recoverAuthorityFromSignatureHeaders(c)
	c.Locals("authedUserId", userId)
	c.Locals("authedWallet", wallet)
	fmt.Println("authMiddleware", userId, wallet)

	myId := app.getMyId(c)
	requestedWallet := c.Params("wallet")

	// Not authorized to act on behalf of myId
	if myId != 0 {
		if userId != myId && !app.isAuthorizedRequest(c.Context(), myId, wallet) {
			return fiber.NewError(
				fiber.StatusForbidden,
				fmt.Sprintf(
					"You are not authorized to make this request authedUserId=%d authedWallet=%s myId=%d",
					userId,
					wallet,
					myId,
				),
			)
		}
	}

	// Not authorized to act on behalf of requestedWallet
	if requestedWallet != "" && wallet != "" {
		if !strings.EqualFold(requestedWallet, wallet) {
			return fiber.NewError(
				fiber.StatusForbidden,
				fmt.Sprintf(
					"You are not authorized to make this request authedUserId=%d authedWallet=%s requestedWallet=%s",
					userId,
					wallet,
					requestedWallet,
				),
			)
		}
	}

	return c.Next()
}

// Middleware that asserts that there is an authedUserId
func (app *ApiServer) requireAuthMiddleware(c *fiber.Ctx) error {
	authedUserId := app.getAuthedUserId(c)
	if authedUserId == 0 {
		return fiber.NewError(fiber.StatusUnauthorized, "You must be logged in to make this request")
	}

	return c.Next()
}
