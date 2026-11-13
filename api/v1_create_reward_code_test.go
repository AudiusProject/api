package api

import (
	"crypto/rand"
	"fmt"
	"testing"

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

func TestV1CreateRewardCodeRetired(t *testing.T) {
	app := testAppWithFixtures(t)
	status, _ := testPost(t, app, "/v1/rewards/code", []byte(`{}`), nil)
	assert.Equal(t, 410, status)
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
