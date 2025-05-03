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
func (app *ApiServer) recoverAuthorityFromSignatureHeaders(c *fiber.Ctx) string {
	message := c.Get("Encoded-Data-Message")
	signature := c.Get("Encoded-Data-Signature")
	if message == "" || signature == "" {
		return ""
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
		return ""
	}

	recoveredAddr := crypto.PubkeyToAddress(*publicKey)
	walletLower := strings.ToLower(recoveredAddr.Hex())

	return walletLower
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
			-- I am the user
			SELECT 1 FROM users
			WHERE
				user_id = $1
				AND wallet = $2
				AND is_current = true

			UNION ALL

			-- I have a grant to the user
			SELECT 1 FROM grants
			WHERE
				is_current = true
				AND user_id = $1
				AND grantee_address = $2
				AND is_approved = true
				AND is_revoked = false
		);
		`, userId, authedWallet).Scan(&isAuthorized)

	if err != nil {
		return false
	}

	app.resolveGrantCache.Set(cacheKey, isAuthorized)
	return isAuthorized
}

func (app *ApiServer) getAuthedWallet(c *fiber.Ctx) string {
	return c.Locals("authedWallet").(string)
}

// Middleware to set authedUserId and authedWallet in context
// Returns a 403 if either
// - the user is not authorized to act on behalf of "myId"
// - the user is not authorized to act on behalf of "myWallet"
func (app *ApiServer) authMiddleware(c *fiber.Ctx) error {
	wallet := app.recoverAuthorityFromSignatureHeaders(c)
	c.Locals("authedWallet", wallet)

	// Not authorized to act on behalf of myId
	myId := app.getMyId(c)
	if myId != 0 && !app.isAuthorizedRequest(c.Context(), myId, wallet) {
		return fiber.NewError(
			fiber.StatusForbidden,
			fmt.Sprintf(
				"You are not authorized to make this request authedWallet=%s myId=%d",
				wallet,
				myId,
			),
		)
	}

	// Not authorized to act on behalf of myWallet
	myWallet := c.Params("wallet")
	if myWallet != "" && !strings.EqualFold(myWallet, wallet) {
		return fiber.NewError(
			fiber.StatusForbidden,
			fmt.Sprintf(
				"You are not authorized to make this request authedWallet=%s myWallet=%s",
				wallet,
				myWallet,
			),
		)
	}

	return c.Next()
}

// Middleware that asserts that there is an authed wallet
func (app *ApiServer) requireAuthMiddleware(c *fiber.Ctx) error {
	authedWallet := app.getAuthedWallet(c)
	if authedWallet == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "You must be logged in to make this request")
	}

	return c.Next()
}
