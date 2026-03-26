package api

import (
	"crypto/ed25519"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSolanaWalletMiddleware(t *testing.T) {
	app := emptyTestApp(t)

	var capturedWallet string
	testApp := fiber.New()
	testApp.Get("/", app.solanaWalletMiddleware, func(c *fiber.Ctx) error {
		capturedWallet = app.tryGetSolanaWallet(c)
		return c.SendStatus(fiber.StatusOK)
	})

	// Generate a fresh ed25519 keypair for testing
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)
	wallet := base58.Encode(pub)
	message := "Sign in to Audius"
	sig := ed25519.Sign(priv, []byte(message))

	t.Run("no headers is a no-op", func(t *testing.T) {
		capturedWallet = ""
		req := httptest.NewRequest("GET", "/", nil)
		res, err := testApp.Test(req, -1)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, res.StatusCode)
		assert.Equal(t, "", capturedWallet)
	})

	t.Run("valid signature sets solanaWallet", func(t *testing.T) {
		capturedWallet = ""
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Solana-Wallet", wallet)
		req.Header.Set("X-Solana-Message", message)
		req.Header.Set("X-Solana-Signature", base58.Encode(sig))
		res, err := testApp.Test(req, -1)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusOK, res.StatusCode)
		assert.Equal(t, wallet, capturedWallet)
	})

	t.Run("partial headers returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Solana-Wallet", wallet)
		// missing message and signature
		res, err := testApp.Test(req, -1)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusUnauthorized, res.StatusCode)
	})

	t.Run("invalid signature returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Solana-Wallet", wallet)
		req.Header.Set("X-Solana-Message", "wrong message")
		req.Header.Set("X-Solana-Signature", base58.Encode(sig))
		res, err := testApp.Test(req, -1)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusUnauthorized, res.StatusCode)
	})

	t.Run("invalid wallet pubkey returns 401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("X-Solana-Wallet", "notavalidkey")
		req.Header.Set("X-Solana-Message", message)
		req.Header.Set("X-Solana-Signature", base58.Encode(sig))
		res, err := testApp.Test(req, -1)
		assert.NoError(t, err)
		assert.Equal(t, fiber.StatusUnauthorized, res.StatusCode)
		body, _ := io.ReadAll(res.Body)
		assert.Contains(t, string(body), "invalid Solana wallet public key")
	})
}
