package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"github.com/gagliardetto/solana-go"
	"github.com/mr-tron/base58"
	"github.com/stretchr/testify/assert"
)

// Helper function to create a valid signature for testing
func createValidSignature(privateKey ed25519.PrivateKey, message string) string {
	signature := ed25519.Sign(privateKey, []byte(message))
	return base58.Encode(signature)
}

func TestV1CreateRewardCode(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":     "TEST",
				"decimals":   8,
				"user_id":    1,
				"mint":       "TestMint123",
				"name":       "Test Coin",
				"created_at": time.Now().Add(-time.Second),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// Save original config and restore after tests
	originalKeys := config.Cfg.RewardCodeAuthorizedKeys
	defer func() {
		config.Cfg.RewardCodeAuthorizedKeys = originalKeys
	}()

	t.Run("Unauthorized with invalid signature", func(t *testing.T) {
		_, wrongPrivateKey, err := ed25519.GenerateKey(rand.Reader)
		assert.NoError(t, err)
		signature := createValidSignature(wrongPrivateKey, "code")

		requestBody := CreateRewardCodeRequest{
			Signature: signature,
			Mint:      "TestMint123",
			Amount:    100,
		}

		body, err := json.Marshal(requestBody)
		assert.NoError(t, err)

		status, _ := testPost(t, app, "/v1/rewards/code", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 403, status, "Should return 403 for unauthorized signature")
	})

	t.Run("Missing required fields", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"mint": "TestMint123",
			// Missing signature and amount
		}

		body, err := json.Marshal(requestBody)
		assert.NoError(t, err)

		status, _ := testPost(t, app, "/v1/rewards/code", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for missing required fields")
	})

	t.Run("Invalid amount (zero)", func(t *testing.T) {
		_, wrongPrivateKey, err := ed25519.GenerateKey(rand.Reader)
		assert.NoError(t, err)
		signature := createValidSignature(wrongPrivateKey, "code")

		requestBody := CreateRewardCodeRequest{
			Signature: signature,
			Mint:      "TestMint123",
			Amount:    0,
		}

		body, err := json.Marshal(requestBody)
		assert.NoError(t, err)

		status, _ := testPost(t, app, "/v1/rewards/code", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for invalid amount")
	})

	t.Run("Successfully creates a reward code", func(t *testing.T) {
		// Generate a test keypair
		testPublicKey, testPrivateKey, err := ed25519.GenerateKey(rand.Reader)
		assert.NoError(t, err)

		// Convert to Solana format and inject into config
		solanaPubKey := solana.PublicKeyFromBytes(testPublicKey)
		testPubKeyBase58 := solanaPubKey.String()
		config.Cfg.RewardCodeAuthorizedKeys = []string{testPubKeyBase58}

		// Create a valid signature
		signature := createValidSignature(testPrivateKey, signedAuthMessage)

		requestBody := CreateRewardCodeRequest{
			Signature: signature,
			Mint:      "TestMint123",
			Amount:    500,
		}

		body, err := json.Marshal(requestBody)
		assert.NoError(t, err)

		var resp CreateRewardCodeResponse
		status, respBody := testPost(t, app, "/v1/rewards/code", body, map[string]string{
			"Content-Type": "application/json",
		}, &resp)
		assert.Equal(t, 201, status, "Response body: %s", string(respBody))
		assert.Equal(t, "TestMint123", resp.Mint)
		assert.Equal(t, int64(500), resp.Amount)
		assert.Len(t, resp.Code, codeLength)
		assert.Regexp(t, "^[a-zA-Z0-9]{10}$", resp.Code)

		// Verify the code exists in the database and remaining_uses is 1
		var dbCode string
		var dbMint string
		var dbAmount int64
		var dbRemainingUses int
		err = app.pool.QueryRow(context.Background(),
			"SELECT code, mint, amount, remaining_uses FROM reward_codes WHERE code = $1", resp.Code).
			Scan(&dbCode, &dbMint, &dbAmount, &dbRemainingUses)
		assert.NoError(t, err)
		assert.Equal(t, resp.Code, dbCode)
		assert.Equal(t, resp.Mint, dbMint)
		assert.Equal(t, resp.Amount, dbAmount)
		assert.Equal(t, 1, dbRemainingUses)
	})

	t.Run("Successfully creates a reward code with second signer", func(t *testing.T) {
		// Generate a test keypair
		testPublicKey, _, err := ed25519.GenerateKey(rand.Reader)
		testPublicKey2, testPrivateKey2, err := ed25519.GenerateKey(rand.Reader)
		assert.NoError(t, err)

		// Convert to Solana format and inject into config
		solanaPubKey := solana.PublicKeyFromBytes(testPublicKey)
		solanaPubKey2 := solana.PublicKeyFromBytes(testPublicKey2)
		testPubKeyBase58 := solanaPubKey.String()
		testPubKeyBase582 := solanaPubKey2.String()
		config.Cfg.RewardCodeAuthorizedKeys = []string{testPubKeyBase58, testPubKeyBase582}

		// Create a valid signature
		signature2 := createValidSignature(testPrivateKey2, signedAuthMessage)
		requestBody := CreateRewardCodeRequest{
			Signature: signature2,
			Mint:      "TestMint123",
			Amount:    500,
		}

		body, err := json.Marshal(requestBody)
		assert.NoError(t, err)

		var resp CreateRewardCodeResponse
		status, respBody := testPost(t, app, "/v1/rewards/code", body, map[string]string{
			"Content-Type": "application/json",
		}, &resp)
		assert.Equal(t, 201, status, "Response body: %s", string(respBody))
		assert.Equal(t, "TestMint123", resp.Mint)
		assert.Equal(t, int64(500), resp.Amount)
		assert.Len(t, resp.Code, codeLength)
		assert.Regexp(t, "^[a-zA-Z0-9]{6}$", resp.Code)

		// Verify the code exists in the database and remaining_uses is 1
		var dbCode string
		var dbMint string
		var dbAmount int64
		var dbRemainingUses int
		err = app.pool.QueryRow(context.Background(),
			"SELECT code, mint, amount, remaining_uses FROM reward_codes WHERE code = $1", resp.Code).
			Scan(&dbCode, &dbMint, &dbAmount, &dbRemainingUses)
		assert.NoError(t, err)
		assert.Equal(t, resp.Code, dbCode)
		assert.Equal(t, resp.Mint, dbMint)
		assert.Equal(t, resp.Amount, dbAmount)
		assert.Equal(t, 1, dbRemainingUses)
	})
}

func TestGenerateCode(t *testing.T) {
	t.Run("Generates 10 character code", func(t *testing.T) {
		code, err := generateCode()
		assert.NoError(t, err)
		assert.Len(t, code, 10)
	})

	t.Run("Generates alphanumeric characters only", func(t *testing.T) {
		code, err := generateCode()
		assert.NoError(t, err)

		for _, char := range code {
			assert.Contains(t, codeChars, string(char), "Code should only contain alphanumeric characters")
		}
	})

	t.Run("Generates different codes", func(t *testing.T) {
		codes := make(map[string]bool)
		iterations := 100

		for i := 0; i < iterations; i++ {
			code, err := generateCode()
			assert.NoError(t, err)
			codes[code] = true
		}

		// With 62^6 possible combinations, we should get mostly unique codes
		assert.Greater(t, len(codes), iterations*9/10, "Should unique codes")
	})
}

func TestVerifySignature(t *testing.T) {
	t.Run("Returns false for invalid signature", func(t *testing.T) {
		_, testPrivateKey, err := ed25519.GenerateKey(rand.Reader)
		assert.NoError(t, err)

		// Create a signature with test key
		signature := createValidSignature(testPrivateKey, "code")

		// Verify against a different key (should fail)
		mockKey := "DDT15s6MMNxE4jkyGN46wNYqrgLWofT6WAvWtjYYrCUq"
		valid, err := verifySignature(signature, mockKey)
		assert.NoError(t, err)
		assert.False(t, valid, "Should return false for signature from wrong key")
	})

	t.Run("Returns error for invalid base58", func(t *testing.T) {
		mockKey := "DDT15s6MMNxE4jkyGN46wNYqrgLWofT6WAvWtjYYrCUq"
		_, err := verifySignature("not-valid-base58!@#", mockKey)
		assert.Error(t, err, "Should return error for invalid base58")
	})

	t.Run("Returns false for wrong message", func(t *testing.T) {
		testPubKey, testPrivateKey, err := ed25519.GenerateKey(rand.Reader)
		assert.NoError(t, err)

		// Convert test public key to Solana base58
		solanaPubKey := solana.PublicKeyFromBytes(testPubKey)
		testPubKeyBase58 := solanaPubKey.String()

		// Sign a different message
		signature := createValidSignature(testPrivateKey, "wrong-message")
		valid, err := verifySignature(signature, testPubKeyBase58)
		assert.NoError(t, err)
		assert.False(t, valid, "Should return false for signature of wrong message")
	})
}
