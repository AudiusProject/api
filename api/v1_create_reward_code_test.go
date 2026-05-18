package api

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
)

// buildSignedMemoTx constructs a transaction containing a single Memo
// Program instruction whose data is the millisecond timestamp encoded as
// UTF-8 decimal digits, signs it with privKey, and returns the base64
// representation suitable for the request body.
func buildSignedMemoTx(t *testing.T, privKey solana.PrivateKey, timestamp int64) string {
	t.Helper()

	memoIx := solana.NewInstruction(
		solana.MemoProgramID,
		solana.AccountMetaSlice{},
		[]byte(fmt.Sprintf("%d", timestamp)),
	)

	// Random dummy blockhash — wallets and our server don't validate it
	// against a live chain since the tx is never submitted.
	var blockhash solana.Hash
	_, err := rand.Read(blockhash[:])
	assert.NoError(t, err)

	tx, err := solana.NewTransaction(
		[]solana.Instruction{memoIx},
		blockhash,
		solana.TransactionPayer(privKey.PublicKey()),
	)
	assert.NoError(t, err)

	_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		if key.Equals(privKey.PublicKey()) {
			return &privKey
		}
		return nil
	})
	assert.NoError(t, err)

	encoded, err := tx.ToBase64()
	assert.NoError(t, err)
	return encoded
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

	originalKeys := app.config.RewardCodeAuthorizedKeys
	defer func() {
		app.config.RewardCodeAuthorizedKeys = originalKeys
	}()

	t.Run("Successfully creates a reward code", func(t *testing.T) {
		privKey, err := solana.NewRandomPrivateKey()
		assert.NoError(t, err)
		app.config.RewardCodeAuthorizedKeys = []string{privKey.PublicKey().String()}

		signedTx := buildSignedMemoTx(t, privKey, time.Now().UnixMilli())

		body, err := json.Marshal(CreateRewardCodeRequest{
			SignedTransaction: signedTx,
			Mint:              "TestMint123",
			Amount:            500,
		})
		assert.NoError(t, err)

		var resp CreateRewardCodeResponse
		status, respBody := testPost(t, app, "/v1/rewards/code", body, map[string]string{
			"Content-Type": "application/json",
		}, &resp)
		assert.Equal(t, 201, status, "Response body: %s", string(respBody))
		assert.Equal(t, "TestMint123", resp.Mint)
		assert.Equal(t, int64(500), resp.Amount)
		assert.Len(t, resp.Code, codeLength)

		var dbCode string
		var dbRemainingUses int
		err = app.pool.QueryRow(context.Background(),
			"SELECT code, remaining_uses FROM reward_codes WHERE code = $1", resp.Code).
			Scan(&dbCode, &dbRemainingUses)
		assert.NoError(t, err)
		assert.Equal(t, 1, dbRemainingUses)
	})

	t.Run("Successfully creates a reward code with second authorized signer", func(t *testing.T) {
		_, err := solana.NewRandomPrivateKey()
		assert.NoError(t, err)
		privKey2, err := solana.NewRandomPrivateKey()
		assert.NoError(t, err)
		other, err := solana.NewRandomPrivateKey()
		assert.NoError(t, err)
		app.config.RewardCodeAuthorizedKeys = []string{other.PublicKey().String(), privKey2.PublicKey().String()}

		signedTx := buildSignedMemoTx(t, privKey2, time.Now().UnixMilli())

		body, err := json.Marshal(CreateRewardCodeRequest{
			SignedTransaction: signedTx,
			Mint:              "TestMint123",
			Amount:            500,
		})
		assert.NoError(t, err)

		var resp CreateRewardCodeResponse
		status, respBody := testPost(t, app, "/v1/rewards/code", body, map[string]string{
			"Content-Type": "application/json",
		}, &resp)
		assert.Equal(t, 201, status, "Response body: %s", string(respBody))
	})

	t.Run("Unauthorized signer returns 403", func(t *testing.T) {
		signerKey, err := solana.NewRandomPrivateKey()
		assert.NoError(t, err)
		otherKey, err := solana.NewRandomPrivateKey()
		assert.NoError(t, err)
		// Authorize a different key than the signer
		app.config.RewardCodeAuthorizedKeys = []string{otherKey.PublicKey().String()}

		signedTx := buildSignedMemoTx(t, signerKey, time.Now().UnixMilli())

		body, err := json.Marshal(CreateRewardCodeRequest{
			SignedTransaction: signedTx,
			Mint:              "TestMint123",
			Amount:            100,
		})
		assert.NoError(t, err)

		status, _ := testPost(t, app, "/v1/rewards/code", body, map[string]string{
			"Content-Type": "application/json",
		})
		assert.Equal(t, 403, status)
	})

	t.Run("Missing signed_transaction returns 400", func(t *testing.T) {
		body, err := json.Marshal(map[string]interface{}{
			"mint":   "TestMint123",
			"amount": 100,
		})
		assert.NoError(t, err)

		status, _ := testPost(t, app, "/v1/rewards/code", body, map[string]string{
			"Content-Type": "application/json",
		})
		assert.Equal(t, 400, status)
	})

	t.Run("Invalid base64 transaction returns 400", func(t *testing.T) {
		privKey, err := solana.NewRandomPrivateKey()
		assert.NoError(t, err)
		app.config.RewardCodeAuthorizedKeys = []string{privKey.PublicKey().String()}

		body, err := json.Marshal(CreateRewardCodeRequest{
			SignedTransaction: "not-valid-base64!@#$",
			Mint:              "TestMint123",
			Amount:            100,
		})
		assert.NoError(t, err)

		status, _ := testPost(t, app, "/v1/rewards/code", body, map[string]string{
			"Content-Type": "application/json",
		})
		assert.Equal(t, 400, status)
	})

	t.Run("Timestamp out of range returns 400", func(t *testing.T) {
		privKey, err := solana.NewRandomPrivateKey()
		assert.NoError(t, err)
		app.config.RewardCodeAuthorizedKeys = []string{privKey.PublicKey().String()}

		// 24 hours in the past — outside the 12h drift window
		staleTimestamp := time.Now().Add(-24 * time.Hour).UnixMilli()
		signedTx := buildSignedMemoTx(t, privKey, staleTimestamp)

		body, err := json.Marshal(CreateRewardCodeRequest{
			SignedTransaction: signedTx,
			Mint:              "TestMint123",
			Amount:            100,
		})
		assert.NoError(t, err)

		status, _ := testPost(t, app, "/v1/rewards/code", body, map[string]string{
			"Content-Type": "application/json",
		})
		assert.Equal(t, 400, status)
	})

	t.Run("Replay prevention: duplicate signature returns 400", func(t *testing.T) {
		privKey, err := solana.NewRandomPrivateKey()
		assert.NoError(t, err)
		app.config.RewardCodeAuthorizedKeys = []string{privKey.PublicKey().String()}

		signedTx := buildSignedMemoTx(t, privKey, time.Now().UnixMilli())

		body, err := json.Marshal(CreateRewardCodeRequest{
			SignedTransaction: signedTx,
			Mint:              "TestMint123",
			Amount:            500,
		})
		assert.NoError(t, err)

		// First request succeeds
		status1, respBody1 := testPost(t, app, "/v1/rewards/code", body, map[string]string{
			"Content-Type": "application/json",
		})
		assert.Equal(t, 201, status1, "First request should succeed. Body: %s", string(respBody1))

		// Same transaction again should be rejected as a duplicate signature
		status2, _ := testPost(t, app, "/v1/rewards/code", body, map[string]string{
			"Content-Type": "application/json",
		})
		assert.Equal(t, 400, status2)
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

		assert.Greater(t, len(codes), iterations*9/10, "Should produce mostly unique codes")
	})
}

func TestExtractMemoTimestamp(t *testing.T) {
	t.Run("extracts timestamp from memo instruction", func(t *testing.T) {
		privKey, err := solana.NewRandomPrivateKey()
		assert.NoError(t, err)

		want := int64(1747251234567)
		signedTx := buildSignedMemoTx(t, privKey, want)

		tx, err := solana.TransactionFromBase64(signedTx)
		assert.NoError(t, err)

		got, err := extractMemoTimestamp(tx)
		assert.NoError(t, err)
		assert.Equal(t, want, got)
	})

	t.Run("returns error when no memo instruction is present", func(t *testing.T) {
		privKey, err := solana.NewRandomPrivateKey()
		assert.NoError(t, err)

		// Build a transaction with a non-memo instruction
		nonMemoIx := solana.NewInstruction(
			solana.SystemProgramID,
			solana.AccountMetaSlice{},
			[]byte{0, 0, 0, 0},
		)
		var blockhash solana.Hash
		_, err = rand.Read(blockhash[:])
		assert.NoError(t, err)
		tx, err := solana.NewTransaction(
			[]solana.Instruction{nonMemoIx},
			blockhash,
			solana.TransactionPayer(privKey.PublicKey()),
		)
		assert.NoError(t, err)
		_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(privKey.PublicKey()) {
				return &privKey
			}
			return nil
		})
		assert.NoError(t, err)

		_, err = extractMemoTimestamp(tx)
		assert.Error(t, err)
	})

	t.Run("returns error when memo data is not a valid integer", func(t *testing.T) {
		privKey, err := solana.NewRandomPrivateKey()
		assert.NoError(t, err)

		memoIx := solana.NewInstruction(
			solana.MemoProgramID,
			solana.AccountMetaSlice{},
			[]byte("not-a-number"),
		)
		var blockhash solana.Hash
		_, err = rand.Read(blockhash[:])
		assert.NoError(t, err)
		tx, err := solana.NewTransaction(
			[]solana.Instruction{memoIx},
			blockhash,
			solana.TransactionPayer(privKey.PublicKey()),
		)
		assert.NoError(t, err)
		_, err = tx.Sign(func(key solana.PublicKey) *solana.PrivateKey {
			if key.Equals(privKey.PublicKey()) {
				return &privKey
			}
			return nil
		})
		assert.NoError(t, err)

		_, err = extractMemoTimestamp(tx)
		assert.Error(t, err)
	})
}
