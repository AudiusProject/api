package api

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
)

const (
	messageHeader   = "Encoded-Data-Message"
	signatureHeader = "Encoded-Data-Signature"
)

// Recover user id and wallet from signature headers
func (app *ApiServer) recoverAuthorityFromSignatureHeaders(c *fiber.Ctx) (int32, string) {
	message := c.Get(messageHeader)
	signature := c.Get(signatureHeader)
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

	var userId int32
	err = app.pool.QueryRow(
		c.Context(),
		`
		SELECT user_id FROM users 
		WHERE
			wallet = $1 
			AND is_current = true 
		ORDER BY created_at ASC 
		LIMIT 1
		`,
		walletLower,
	).Scan(&userId)

	if err != nil {
		return 0, walletLower
	}

	return userId, walletLower
}

func (app *ApiServer) getAuthedUserId(c *fiber.Ctx) int32 {
	return int32(c.Locals("authedUserId").(int32))
}

func (app *ApiServer) getAuthedWallet(c *fiber.Ctx) string {
	return c.Locals("authedWallet").(string)
}

// Middleware to set authedUserId and authedWallet in context
func (app *ApiServer) authMiddleware(c *fiber.Ctx) error {
	userId, wallet := app.recoverAuthorityFromSignatureHeaders(c)
	c.Locals("authedUserId", userId)
	c.Locals("authedWallet", wallet)

	return c.Next()
}

// Middleware that asserts authedUserId is valid
func (app *ApiServer) requiresAuthMiddleware(c *fiber.Ctx) error {
	authedUserId := app.getAuthedUserId(c)
	if authedUserId == 0 {
		return fiber.NewError(fiber.StatusUnauthorized, "You must be logged in to make this request")
	}
	return c.Next()
}
