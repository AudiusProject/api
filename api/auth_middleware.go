package api

import (
	"context"
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/jackc/pgx/v5"
)

// Recover user id and wallet from signature headers
func (app *ApiServer) recoverAuthorityFromSignatureHeaders(c *fiber.Ctx) string {
	message := c.Get("Encoded-Data-Message", c.Query("user_data"))
	signature := c.Get("Encoded-Data-Signature", c.Query("user_signature"))
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
    ) OR EXISTS (
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

// Get a user from their wallet address.
//
// Note: Do NOT use this with `getAuthedWallet()` to infer the current user.
// Comms/chats _does_ use this to infer current user due to legacy reasons, as
// it predates manager mode and messages are e2ee via wallet. V1 endpoints
// should use the query or route parameters for determining the current user.
func (app *ApiServer) getUserIDFromWallet(ctx context.Context, wallet string) (int, error) {
	key := utils.CopyString(wallet)
	if hit, ok := app.resolveWalletCache.Get(key); ok {
		return hit, nil
	}

	sql := `
	SELECT user_id
	FROM users
	WHERE is_current = true
		AND handle IS NOT NULL
		AND is_available = true
		AND is_deactivated = false
		AND wallet = LOWER(@wallet)
	ORDER BY user_id ASC;
	`
	row := app.pool.QueryRow(ctx, sql, pgx.NamedArgs{
		"wallet": wallet,
	})

	userId := 0
	err := row.Scan(&userId)
	if err != nil {
		return 0, fiber.NewError(fiber.ErrBadRequest.Code, "bad signature")
	}

	app.resolveWalletCache.Set(key, userId)
	return userId, nil
}
