package api

import (
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

// Helper function to create a test JWT token
func createTestJWT(privateKey *ecdsa.PrivateKey, payload map[string]interface{}) (string, error) {
	// Create header
	header := map[string]interface{}{
		"typ": "JWT",
		"alg": "ES256K",
	}

	headerBytes, err := json.Marshal(header)
	if err != nil {
		return "", err
	}
	base64Header := base64.URLEncoding.EncodeToString(headerBytes)
	base64Header = strings.TrimRight(base64Header, "=")

	// Create payload
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	base64Payload := base64.URLEncoding.EncodeToString(payloadBytes)
	base64Payload = strings.TrimRight(base64Payload, "=")

	// Create message to sign
	message := fmt.Sprintf("%s.%s", base64Header, base64Payload)

	// Create ethereum signed message format
	encodedToRecover := []byte(message)
	prefixedMessage := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(encodedToRecover), encodedToRecover))
	finalHash := crypto.Keccak256Hash(prefixedMessage)

	// Sign the message
	signature, err := crypto.Sign(finalHash.Bytes(), privateKey)
	if err != nil {
		return "", err
	}

	// Convert signature to hex string, then base64 encode the hex string
	signatureHex := fmt.Sprintf("0x%x", signature)
	base64Signature := base64.URLEncoding.EncodeToString([]byte(signatureHex))
	base64Signature = strings.TrimRight(base64Signature, "=")

	return fmt.Sprintf("%s.%s.%s", base64Header, base64Payload, base64Signature), nil
}

func TestUsersVerifyTokenValidSignature(t *testing.T) {
	app := emptyTestApp(t)

	// Create test private key and wallet
	privateKey, err := crypto.GenerateKey()
	assert.NoError(t, err)

	wallet := crypto.PubkeyToAddress(privateKey.PublicKey)
	walletLower := strings.ToLower(wallet.Hex())

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id": 1,
				"wallet":  walletLower,
				"handle":  "testuser",
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// Create JWT payload
	payload := map[string]interface{}{
		"userId": trashid.MustEncodeHashID(1),
		"iat":    time.Now().Unix(),
	}

	// Create JWT token
	token, err := createTestJWT(privateKey, payload)
	assert.NoError(t, err)

	// Test the endpoint
	status, body := testGet(t, app, "/v1/users/verify_token?token="+token)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.userId": trashid.MustEncodeHashID(1),
		"data.iat":    fmt.Sprintf("%d", payload["iat"].(int64)),
	})
}

func TestUsersVerifyTokenWithManager(t *testing.T) {
	app := emptyTestApp(t)

	// Create test private keys and wallets
	userPrivateKey, err := crypto.GenerateKey()
	assert.NoError(t, err)
	userWallet := crypto.PubkeyToAddress(userPrivateKey.PublicKey)
	userWalletLower := strings.ToLower(userWallet.Hex())

	managerPrivateKey, err := crypto.GenerateKey()
	assert.NoError(t, err)
	managerWallet := crypto.PubkeyToAddress(managerPrivateKey.PublicKey)
	managerWalletLower := strings.ToLower(managerWallet.Hex())

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id": 1,
				"wallet":  userWalletLower,
				"handle":  "testuser",
			},
			{
				"user_id": 2,
				"wallet":  managerWalletLower,
				"handle":  "manager",
			},
		},
		"grants": {
			{
				"user_id":         1,
				"grantee_address": managerWalletLower,
				"is_revoked":      false,
				"is_approved":     true,
				"is_current":      true,
				"created_at":      time.Now(),
				"updated_at":      time.Now(),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// Create JWT payload for user 1 but sign with manager's key
	payload := map[string]interface{}{
		"userId": trashid.MustEncodeHashID(1),
		"iat":    time.Now().Unix(),
	}

	// Create JWT token signed by manager
	token, err := createTestJWT(managerPrivateKey, payload)
	assert.NoError(t, err)

	// Test the endpoint
	status, body := testGet(t, app, "/v1/users/verify_token?token="+token)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.userId": trashid.MustEncodeHashID(1),
		"data.iat":    fmt.Sprintf("%d", payload["iat"].(int64)),
	})
}

func TestUsersVerifyTokenInvalidJWTFormat(t *testing.T) {
	app := emptyTestApp(t)

	// Test with invalid token format
	status, _ := testGet(t, app, "/v1/users/verify_token?token=invalid.token")
	assert.Equal(t, 400, status)
}

func TestUsersVerifyTokenMissingToken(t *testing.T) {
	app := emptyTestApp(t)

	// Test without token parameter
	status, _ := testGet(t, app, "/v1/users/verify_token")
	assert.Equal(t, 400, status)
}

func TestUsersVerifyTokenInvalidSignature(t *testing.T) {
	app := emptyTestApp(t)

	// Create a valid header and payload but invalid signature
	header := map[string]interface{}{
		"typ": "JWT",
		"alg": "ES256K",
	}
	headerBytes, _ := json.Marshal(header)
	base64Header := base64.URLEncoding.EncodeToString(headerBytes)
	base64Header = strings.TrimRight(base64Header, "=")

	payload := map[string]interface{}{
		"userId": trashid.MustEncodeHashID(1),
		"iat":    time.Now().Unix(),
	}
	payloadBytes, _ := json.Marshal(payload)
	base64Payload := base64.URLEncoding.EncodeToString(payloadBytes)
	base64Payload = strings.TrimRight(base64Payload, "=")

	// Use invalid signature
	invalidSignature := "invalidsignature"
	token := fmt.Sprintf("%s.%s.%s", base64Header, base64Payload, invalidSignature)

	status, _ := testGet(t, app, "/v1/users/verify_token?token="+token)
	assert.Equal(t, 400, status)
}

func TestUsersVerifyTokenWalletNotFound(t *testing.T) {
	app := emptyTestApp(t)

	// Create test private key but don't add user to database
	privateKey, err := crypto.GenerateKey()
	assert.NoError(t, err)

	payload := map[string]interface{}{
		"userId": trashid.MustEncodeHashID(1),
		"iat":    time.Now().Unix(),
	}

	token, err := createTestJWT(privateKey, payload)
	assert.NoError(t, err)

	status, _ := testGet(t, app, "/v1/users/verify_token?token="+token)
	assert.Equal(t, 404, status)
}

func TestUsersVerifyTokenUnauthorizedWallet(t *testing.T) {
	app := emptyTestApp(t)

	// Create two different private keys
	user1PrivateKey, err := crypto.GenerateKey()
	assert.NoError(t, err)
	user1Wallet := crypto.PubkeyToAddress(user1PrivateKey.PublicKey)
	user1WalletLower := strings.ToLower(user1Wallet.Hex())

	user2PrivateKey, err := crypto.GenerateKey()
	assert.NoError(t, err)
	user2Wallet := crypto.PubkeyToAddress(user2PrivateKey.PublicKey)
	user2WalletLower := strings.ToLower(user2Wallet.Hex())

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id": 1,
				"wallet":  user1WalletLower,
				"handle":  "user1",
			},
			{
				"user_id": 2,
				"wallet":  user2WalletLower,
				"handle":  "user2",
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// Create JWT payload for user 1 but sign with user 2's key (without manager relationship)
	payload := map[string]interface{}{
		"userId": trashid.MustEncodeHashID(1),
		"iat":    time.Now().Unix(),
	}

	token, err := createTestJWT(user2PrivateKey, payload)
	assert.NoError(t, err)

	status, _ := testGet(t, app, "/v1/users/verify_token?token="+token)
	assert.Equal(t, 403, status)
}

func TestUsersVerifyTokenInvalidPayload(t *testing.T) {
	app := emptyTestApp(t)

	privateKey, err := crypto.GenerateKey()
	assert.NoError(t, err)
	wallet := crypto.PubkeyToAddress(privateKey.PublicKey)
	walletLower := strings.ToLower(wallet.Hex())

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id": 1,
				"wallet":  walletLower,
				"handle":  "testuser",
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// Create JWT payload without userId
	payload := map[string]interface{}{
		"iat": time.Now().Unix(),
	}

	token, err := createTestJWT(privateKey, payload)
	assert.NoError(t, err)

	status, _ := testGet(t, app, "/v1/users/verify_token?token="+token)
	assert.Equal(t, 400, status)
}
