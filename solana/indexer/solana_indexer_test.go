package indexer_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"bridgerton.audius.co/solana/indexer"
	"bridgerton.audius.co/solana/spl/programs/claimable_tokens"
	"bridgerton.audius.co/solana/spl/programs/payment_router"
	"bridgerton.audius.co/solana/spl/programs/reward_manager"
	"bridgerton.audius.co/solana/spl/programs/secp256k1"
	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/memo"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
	"github.com/test-go/testify/assert"
	"go.uber.org/zap"
)

type mockDBCall struct {
	sql  string
	args any
}

// Mock DBTX that records calls and args
type mockDB struct {
	calls []mockDBCall
}

type mockRow struct {
	values []any
}

func (m mockRow) Scan(dest ...any) error {
	for d := range dest {
		dest[d] = m.values[d]
	}
	return nil
}

// Add stubs for other DBTX methods if needed for compilation
func (m *mockDB) Exec(_ context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if m.calls == nil {
		m.calls = make([]mockDBCall, 0)
	}
	m.calls = append(m.calls, mockDBCall{
		sql:  sql,
		args: args[0],
	})
	return pgconn.CommandTag{}, nil
}
func (m *mockDB) Query(context.Context, string, ...interface{}) (pgx.Rows, error) { return nil, nil }
func (m *mockDB) QueryRow(context.Context, string, ...interface{}) pgx.Row {
	return mockRow{
		values: []any{0},
	}
}

func TestProcessTransaction_CallsInsertClaimableAccount(t *testing.T) {
	mockDb := &mockDB{}
	s := &indexer.SolanaIndexer{}

	// Create a valid CreateTokenAccount instruction
	ethAddress := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")
	payer, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	createInst, err := claimable_tokens.NewCreateTokenAccountInstruction(ethAddress, mint, payer.PublicKey())
	require.NoError(t, err)
	inst, err := createInst.ValidateAndBuild()
	require.NoError(t, err)

	// Compose the transaction message
	tx, err := solana.NewTransactionBuilder().AddInstruction(inst).Build()
	require.NoError(t, err)

	_, err = tx.Sign(func(publicKey solana.PublicKey) *solana.PrivateKey {
		return &payer
	})
	require.NoError(t, err)

	meta := &rpc.TransactionMeta{
		LoadedAddresses: rpc.LoadedAddresses{
			Writable: []solana.PublicKey{},
			ReadOnly: []solana.PublicKey{},
		},
	}

	logger := zap.NewNop()
	ctx := t.Context()
	slot := uint64(1)
	blockTime := time.Now()

	expectedArgs := pgx.NamedArgs{
		"signature":        tx.Signatures[0].String(),
		"instructionIndex": 0,
		"slot":             slot,
		"mint":             mint.String(),
		"ethereumAddress":  strings.ToLower(ethAddress.String()),
		"bankAccount":      createInst.UserBank().PublicKey.String(),
	}

	err = s.ProcessTransaction(ctx, mockDb, slot, meta, tx, blockTime, logger)
	require.NoError(t, err)
	require.Len(t, mockDb.calls, 1)
	require.Contains(t, mockDb.calls[0].sql, "sol_claimable_accounts")
	require.Equal(t, expectedArgs, mockDb.calls[0].args.(pgx.NamedArgs))
}

func TestProcessTransaction_CallsInsertClaimableAccountTransfer(t *testing.T) {
	mockDb := &mockDB{}
	s := &indexer.SolanaIndexer{}

	// Create a valid CreateTokenAccount instruction
	ethAddress := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")
	payer, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	destination, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	transferInst, err := claimable_tokens.NewTransferInstruction(ethAddress, mint, payer.PublicKey(), destination.PublicKey())
	require.NoError(t, err)
	inst, err := transferInst.ValidateAndBuild()
	require.NoError(t, err)

	amount := uint64(1)
	nonce := uint64(2)

	// Create Secp256k1 inst
	msg := claimable_tokens.SignedTransferData{
		Nonce:       nonce,
		Destination: destination.PublicKey(),
		Amount:      amount,
	}
	message := &bytes.Buffer{}
	err = bin.NewBinEncoder(message).Encode(msg)
	require.NoError(t, err)
	secp := secp256k1.NewSecp256k1Instruction(
		ethAddress,
		message.Bytes(),
		[]byte{}, // Doesn't matter
		0,
	).Build()

	// Compose the transaction message
	tx, err := solana.NewTransactionBuilder().
		AddInstruction(secp).
		AddInstruction(inst).
		SetFeePayer(payer.PublicKey()).
		Build()
	require.NoError(t, err)

	_, err = tx.Sign(func(publicKey solana.PublicKey) *solana.PrivateKey {
		return &payer
	})
	require.NoError(t, err)

	meta := &rpc.TransactionMeta{
		LoadedAddresses: rpc.LoadedAddresses{
			Writable: []solana.PublicKey{},
			ReadOnly: []solana.PublicKey{},
		},
	}

	logger := zap.NewNop()
	ctx := t.Context()
	slot := uint64(1)
	blockTime := time.Now()

	expectedArgs := pgx.NamedArgs{
		"signature":        tx.Signatures[0].String(),
		"instructionIndex": 1,
		"amount":           amount,
		"slot":             slot,
		"fromAccount":      transferInst.SenderUserBank().PublicKey.String(),
		"toAccount":        transferInst.Destination().PublicKey.String(),
		"senderEthAddress": strings.ToLower(ethAddress.String()),
	}

	err = s.ProcessTransaction(ctx, mockDb, slot, meta, tx, blockTime, logger)
	assert.NoError(t, err)
	assert.Len(t, mockDb.calls, 1)
	assert.Contains(t, mockDb.calls[0].sql, "sol_claimable_account_transfers")
	assert.Equal(t, expectedArgs, mockDb.calls[0].args.(pgx.NamedArgs))
}

func TestProcessTransaction_CallsInsertRewardDisbursement(t *testing.T) {
	mockDb := &mockDB{}
	s := &indexer.SolanaIndexer{}

	// Setup EvaluateAttestation instruction
	ethAddress := common.HexToAddress("0x3f6d9fcf0d4466dd5886e3b1def017adfb7916b4")
	rewardState := solana.MustPublicKeyFromBase58("GaiG9LDYHfZGqeNaoGRzFEnLiwUT7WiC6sA6FDJX9ZPq")
	destinationUserBank := solana.MustPublicKeyFromBase58("Cjv8dvVfWU8wUYAR82T5oZ4nHLB6EyGNvpPBzw3r76Qy")
	authority := solana.MustPublicKeyFromBase58("6mpecd6bJCpH8oDwwjqPzTPU6QacnwW3cR9pAwEwkYJa")
	tokenSource := solana.MustPublicKeyFromBase58("HJQj8P47BdA7ugjQEn45LaESYrxhiZDygmukt8iumFZJ")
	payer, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)
	disbursement := solana.MustPublicKeyFromBase58("3qQfuDEBWEmxRo5G4J2a4eYUVf9u1LWzLgRPndiwew2w")
	oracle := solana.MustPublicKeyFromBase58("FNz5mur7EFh1LyH5HDaKyWVx7vcfGK6gRizEpDqMfgGk")
	amount := uint64(200000000)
	disbursementId := "ft:37364e80"

	inst := reward_manager.NewEvaluateAttestationInstructionBuilder().
		SetDisbursementId(disbursementId).
		SetRecipientEthAddress(ethAddress).
		SetAmount(amount).
		SetAttestationsAccount(rewardState).
		SetRewardManagerStateAccount(rewardState).
		SetAuthorityAccount(authority).
		SetTokenSourceAccount(tokenSource).
		SetDestinationUserBankAccount(destinationUserBank).
		SetDisbursementAccount(disbursement).
		SetAntiAbuseOracleAccount(oracle).
		SetPayerAccount(payer.PublicKey())
	require.NoError(t, inst.Validate())

	tx, err := solana.NewTransactionBuilder().
		AddInstruction(inst.Build()).
		Build()
	require.NoError(t, err)

	signatures, err := tx.Sign(func(publicKey solana.PublicKey) *solana.PrivateKey {
		return &payer
	})
	require.NoError(t, err)

	meta := &rpc.TransactionMeta{
		LoadedAddresses: rpc.LoadedAddresses{
			Writable: []solana.PublicKey{},
			ReadOnly: []solana.PublicKey{},
		},
	}

	logger := zap.NewNop()
	ctx := t.Context()
	slot := uint64(1)
	blockTime := time.Now()

	expectedArgs := pgx.NamedArgs{
		"signature":        signatures[0].String(),
		"instructionIndex": 0,
		"amount":           amount,
		"slot":             slot,
		"userBank":         destinationUserBank.String(),
		"challengeId":      "ft",
		"specifier":        "37364e80",
	}

	err = s.ProcessTransaction(ctx, mockDb, slot, meta, tx, blockTime, logger)
	assert.NoError(t, err)
	assert.Len(t, mockDb.calls, 1)
	assert.Contains(t, mockDb.calls[0].sql, "sol_reward_disbursements")
	assert.Equal(t, expectedArgs, mockDb.calls[0].args.(pgx.NamedArgs))
}

func TestProcessTransaction_CallsInsertPayment(t *testing.T) {
	mockDb := &mockDB{}
	s := &indexer.SolanaIndexer{}

	// Setup Route instruction
	sender, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)

	dest, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)

	amount := uint64(1000)
	routeInst := payment_router.NewRouteInstruction(
		sender.PublicKey(),
		sender.PublicKey(),
		uint8(0),
		map[solana.PublicKey]uint64{
			dest.PublicKey(): amount,
		},
	).Build()

	payer, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)

	tx, err := solana.NewTransactionBuilder().
		AddInstruction(routeInst).
		SetFeePayer(payer.PublicKey()).
		Build()
	require.NoError(t, err)

	signatures, err := tx.Sign(func(publicKey solana.PublicKey) *solana.PrivateKey {
		return &payer
	})
	require.NoError(t, err)

	meta := &rpc.TransactionMeta{
		LoadedAddresses: rpc.LoadedAddresses{
			Writable: []solana.PublicKey{},
			ReadOnly: []solana.PublicKey{},
		},
	}

	logger := zap.NewNop()
	ctx := t.Context()
	slot := uint64(1)
	blockTime := time.Now()

	expectedArgs := pgx.NamedArgs{
		"signature":        signatures[0].String(),
		"instructionIndex": 0,
		"amount":           amount,
		"slot":             slot,
		"routeIndex":       0,
		"toAccount":        dest.PublicKey().String(),
	}

	err = s.ProcessTransaction(ctx, mockDb, slot, meta, tx, blockTime, logger)
	require.NoError(t, err)
	require.Len(t, mockDb.calls, 1)
	require.Contains(t, mockDb.calls[0].sql, "sol_payments")
	require.Equal(t, expectedArgs, mockDb.calls[0].args.(pgx.NamedArgs))
}

func TestProcessTransaction_CallsInsertPurchase(t *testing.T) {
	mockDb := &mockDB{}
	s := &indexer.SolanaIndexer{}

	// Setup Route instruction
	sender, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)

	dest, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)

	amount := uint64(1000)
	routeInst := payment_router.NewRouteInstruction(
		sender.PublicKey(),
		sender.PublicKey(),
		uint8(0),
		map[solana.PublicKey]uint64{
			dest.PublicKey(): amount,
		},
	).Build()

	purchaseMemoInst := memo.NewMemoInstruction(
		[]byte("track:1:100:2:stream"),
		sender.PublicKey(),
	).Build()

	geoMemoInst := memo.NewMemoInstruction(
		[]byte(`geo:{"city":"Minneapolis","region":"MN","country":"USA"}`),
		sender.PublicKey(),
	).Build()

	payer, err := solana.NewRandomPrivateKey()
	require.NoError(t, err)

	tx, err := solana.NewTransactionBuilder().
		AddInstruction(routeInst).
		AddInstruction(purchaseMemoInst).
		AddInstruction(geoMemoInst).
		SetFeePayer(payer.PublicKey()).
		Build()
	require.NoError(t, err)

	signatures, err := tx.Sign(func(publicKey solana.PublicKey) *solana.PrivateKey {
		return &payer
	})
	require.NoError(t, err)

	meta := &rpc.TransactionMeta{
		LoadedAddresses: rpc.LoadedAddresses{
			Writable: []solana.PublicKey{},
			ReadOnly: []solana.PublicKey{},
		},
	}

	logger := zap.NewNop()
	ctx := t.Context()
	slot := uint64(1)
	blockTime := time.Now()

	expectedPaymentArgs := pgx.NamedArgs{
		"signature":        signatures[0].String(),
		"instructionIndex": 0,
		"amount":           amount,
		"slot":             slot,
		"routeIndex":       0,
		"toAccount":        dest.PublicKey().String(),
	}

	expectedPurchaseArgs := pgx.NamedArgs{
		"signature":             signatures[0].String(),
		"instructionIndex":      0,
		"amount":                amount,
		"slot":                  slot,
		"fromAccount":           sender.PublicKey().String(),
		"contentType":           "track",
		"contentId":             1,
		"buyerUserId":           2,
		"accessType":            "stream",
		"validAfterBlocknumber": 100,
		"isValid":               (*bool)(nil),
		"city":                  "Minneapolis",
		"region":                "MN",
		"country":               "USA",
	}

	err = s.ProcessTransaction(ctx, mockDb, slot, meta, tx, blockTime, logger)
	assert.NoError(t, err)
	assert.Len(t, mockDb.calls, 2)
	assert.Contains(t, mockDb.calls[0].sql, "sol_payments")
	assert.Equal(t, expectedPaymentArgs, mockDb.calls[0].args.(pgx.NamedArgs))
	assert.Contains(t, mockDb.calls[1].sql, "sol_purchases")
	assert.Equal(t, expectedPurchaseArgs, mockDb.calls[1].args.(pgx.NamedArgs))
}

func TestProcessTransaction_CallsInsertBalanceChange(t *testing.T) {
	mockDb := &mockDB{}
	s := &indexer.SolanaIndexer{}

	account := solana.MustPublicKeyFromBase58("HJQj8P47BdA7ugjQEn45LaESYrxhiZDygmukt8iumFZJ")
	account2 := solana.MustPublicKeyFromBase58("Cjv8dvVfWU8wUYAR82T5oZ4nHLB6EyGNvpPBzw3r76Qy")
	mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")
	tx := &solana.Transaction{
		Signatures: []solana.Signature{
			solana.MustSignatureFromBase58("5ZVE83uvxQ36BmUM4kPn2foPyQCbsEepEkDTinC8bfSwHJdVCia6q3Wvnfa2Ls71SZoBmqoWPyJuPuUm8XcG92Hr"),
		},
		Message: solana.Message{
			AccountKeys: []solana.PublicKey{
				account,
				account2,
			},
		},
	}
	meta := &rpc.TransactionMeta{
		PreTokenBalances: []rpc.TokenBalance{
			{
				AccountIndex: 0,
				Mint:         mint,
				UiTokenAmount: &rpc.UiTokenAmount{
					Amount: "1000",
				},
			},
		},
		PostTokenBalances: []rpc.TokenBalance{
			{
				AccountIndex: 0,
				Mint:         mint,
				UiTokenAmount: &rpc.UiTokenAmount{
					Amount: "2000",
				},
			},
			{
				AccountIndex: 1,
				Mint:         mint,
				UiTokenAmount: &rpc.UiTokenAmount{
					Amount: "0",
				},
			},
		},
		LoadedAddresses: rpc.LoadedAddresses{
			Writable: []solana.PublicKey{},
			ReadOnly: []solana.PublicKey{},
		},
	}

	logger := zap.NewNop()
	ctx := t.Context()
	slot := uint64(1)
	blockTime := time.Now()

	expectedArgs := pgx.NamedArgs{
		"account_address": account.String(),
		"mint":            mint.String(),
		"change":          int64(1000),
		"balance":         uint64(2000),
		"signature":       tx.Signatures[0].String(),
		"slot":            slot,
	}

	expectedArgs2 := pgx.NamedArgs{
		"account_address": account2.String(),
		"mint":            mint.String(),
		"change":          int64(0),
		"balance":         uint64(0),
		"signature":       tx.Signatures[0].String(),
		"slot":            slot,
	}

	err := s.ProcessTransaction(ctx, mockDb, slot, meta, tx, blockTime, logger)
	assert.NoError(t, err)
	assert.Len(t, mockDb.calls, 2)
	assert.Contains(t, mockDb.calls[0].sql, "solana_token_txs")
	assert.Equal(t, expectedArgs, mockDb.calls[0].args.(pgx.NamedArgs))
	assert.Contains(t, mockDb.calls[1].sql, "solana_token_txs")
	assert.Equal(t, expectedArgs2, mockDb.calls[1].args.(pgx.NamedArgs))
}
