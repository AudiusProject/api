package api

import (
	"crypto/ed25519"

	"github.com/gofiber/fiber/v2"
	"github.com/mr-tron/base58"
	"go.uber.org/zap"
)

// solanaWalletMiddleware verifies Solana wallet signatures from request headers.
// If the X-Solana-Wallet, X-Solana-Message, and X-Solana-Signature headers are
// present and valid, the verified wallet public key is stored in c.Locals("solanaWallet").
// If headers are absent, the middleware is a no-op. If present but invalid, returns 401.
func (app *ApiServer) solanaWalletMiddleware(c *fiber.Ctx) error {
	wallet := c.Get("X-Solana-Wallet")
	message := c.Get("X-Solana-Message")
	signature := c.Get("X-Solana-Signature")

	// No Solana headers — skip silently
	if wallet == "" && message == "" && signature == "" {
		return c.Next()
	}

	// Partial headers — reject
	if wallet == "" || message == "" || signature == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "incomplete Solana wallet headers: X-Solana-Wallet, X-Solana-Message, and X-Solana-Signature are all required")
	}

	// Decode the base58 public key (32 bytes for ed25519)
	pubkeyBytes, err := base58.Decode(wallet)
	if err != nil || len(pubkeyBytes) != ed25519.PublicKeySize {
		app.logger.Warn("solanaWalletMiddleware: invalid wallet public key", zap.String("wallet", wallet), zap.Error(err))
		return fiber.NewError(fiber.StatusUnauthorized, "invalid Solana wallet public key")
	}

	// Decode the base58 signature (64 bytes for ed25519)
	sigBytes, err := base58.Decode(signature)
	if err != nil || len(sigBytes) != ed25519.SignatureSize {
		app.logger.Warn("solanaWalletMiddleware: invalid signature", zap.Error(err))
		return fiber.NewError(fiber.StatusUnauthorized, "invalid Solana signature")
	}

	// Verify the ed25519 signature
	if !ed25519.Verify(pubkeyBytes, []byte(message), sigBytes) {
		app.logger.Warn("solanaWalletMiddleware: signature verification failed", zap.String("wallet", wallet))
		return fiber.NewError(fiber.StatusUnauthorized, "Solana signature verification failed")
	}

	app.logger.Debug("solanaWalletMiddleware: verified", zap.String("wallet", wallet))
	c.Locals("solanaWallet", wallet)
	return c.Next()
}

// tryGetSolanaWallet returns the verified Solana wallet from context, or "" if not set.
func (app *ApiServer) tryGetSolanaWallet(c *fiber.Ctx) string {
	if w, ok := c.Locals("solanaWallet").(string); ok {
		return w
	}
	return ""
}
