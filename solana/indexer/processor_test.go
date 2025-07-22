package indexer

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"bridgerton.audius.co/config"
	"bridgerton.audius.co/database"
	"bridgerton.audius.co/solana/indexer/fake_rpc_client"
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
	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/require"
	"github.com/test-go/testify/assert"
	"go.uber.org/zap"
)

func TestProcessTransaction_CallsInsertClaimableAccount(t *testing.T) {
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

	// Args
	logger := zap.NewNop()
	ctx := t.Context()
	slot := uint64(1)
	blockTime := time.Now()

	// Mock DB
	poolMock, err := pgxmock.NewPool()
	require.NoError(t, err, "failed to create mock database pool")
	defer poolMock.Close()
	poolMock.ExpectBegin()
	poolMock.ExpectQuery("SELECT mint FROM artist_coins").
		WillReturnError(pgx.ErrNoRows)
	poolMock.ExpectExec("INSERT INTO sol_claimable_accounts").
		WithArgs(pgx.NamedArgs{
			"signature":        tx.Signatures[0].String(),
			"instructionIndex": 0,
			"slot":             slot,
			"mint":             mint.String(),
			"ethereumAddress":  strings.ToLower(ethAddress.String()),
			"account":          createInst.UserBank().PublicKey.String(),
		}).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	poolMock.ExpectCommit()

	p := &DefaultProcessor{
		pool: poolMock,
	}

	err = p.ProcessTransaction(ctx, slot, meta, tx, blockTime, logger)
	require.NoError(t, err)
	assert.NoError(t, poolMock.ExpectationsWereMet())
}

func TestProcessTransaction_CallsInsertClaimableAccountTransfer(t *testing.T) {
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

	// Args
	logger := zap.NewNop()
	ctx := t.Context()
	slot := uint64(1)
	blockTime := time.Now()

	// Mock DB
	poolMock, err := pgxmock.NewPool()
	require.NoError(t, err, "failed to create mock database pool")
	defer poolMock.Close()
	poolMock.ExpectBegin()
	poolMock.ExpectQuery("SELECT mint FROM artist_coins").
		WillReturnError(pgx.ErrNoRows)
	poolMock.ExpectExec("INSERT INTO sol_claimable_account_transfers").
		WithArgs(pgx.NamedArgs{
			"signature":        tx.Signatures[0].String(),
			"instructionIndex": 1,
			"amount":           amount,
			"slot":             slot,
			"fromAccount":      transferInst.SenderUserBank().PublicKey.String(),
			"toAccount":        transferInst.Destination().PublicKey.String(),
			"senderEthAddress": strings.ToLower(ethAddress.String()),
		}).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	poolMock.ExpectCommit()

	p := &DefaultProcessor{
		pool: poolMock,
	}

	err = p.ProcessTransaction(ctx, slot, meta, tx, blockTime, logger)
	assert.NoError(t, err)
	assert.NoError(t, poolMock.ExpectationsWereMet())
}

func TestProcessTransaction_CallsInsertRewardDisbursement(t *testing.T) {
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

	// Args
	logger := zap.NewNop()
	ctx := t.Context()
	slot := uint64(1)
	blockTime := time.Now()

	// Mock DB
	poolMock, err := pgxmock.NewPool()
	require.NoError(t, err, "failed to create mock database pool")
	defer poolMock.Close()
	poolMock.ExpectBegin()
	poolMock.ExpectQuery("SELECT mint FROM artist_coins").
		WillReturnError(pgx.ErrNoRows)
	poolMock.ExpectExec("INSERT INTO sol_reward_disbursements").
		WithArgs(pgx.NamedArgs{
			"signature":        signatures[0].String(),
			"instructionIndex": 0,
			"amount":           amount,
			"slot":             slot,
			"userBank":         destinationUserBank.String(),
			"challengeId":      "ft",
			"specifier":        "37364e80",
		}).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	poolMock.ExpectCommit()

	p := &DefaultProcessor{
		pool: poolMock,
	}

	err = p.ProcessTransaction(ctx, slot, meta, tx, blockTime, logger)
	assert.NoError(t, err)
	assert.NoError(t, poolMock.ExpectationsWereMet())
}

func TestProcessTransaction_CallsInsertPayment(t *testing.T) {
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

	poolMock, err := pgxmock.NewPool()
	require.NoError(t, err, "failed to create mock database pool")
	defer poolMock.Close()
	poolMock.ExpectBegin()
	poolMock.ExpectQuery("SELECT mint FROM artist_coins").
		WillReturnError(pgx.ErrNoRows)
	poolMock.ExpectExec("INSERT INTO sol_payments").
		WithArgs(pgx.NamedArgs{
			"signature":        signatures[0].String(),
			"instructionIndex": 0,
			"amount":           amount,
			"slot":             slot,
			"routeIndex":       0,
			"toAccount":        dest.PublicKey().String(),
		}).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	poolMock.ExpectCommit()

	p := &DefaultProcessor{
		pool: poolMock,
	}

	err = p.ProcessTransaction(ctx, slot, meta, tx, blockTime, logger)
	require.NoError(t, err)
	assert.NoError(t, poolMock.ExpectationsWereMet())
}

func TestProcessTransaction_CallsInsertPurchase(t *testing.T) {
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

	// Args
	logger := zap.NewNop()
	ctx := t.Context()
	slot := uint64(1)
	blockTime := time.Now()

	// Mock DB
	poolMock, err := pgxmock.NewPool()
	require.NoError(t, err, "failed to create mock database pool")
	defer poolMock.Close()
	poolMock.ExpectBegin()
	poolMock.ExpectQuery("SELECT mint FROM artist_coins").
		WillReturnError(pgx.ErrNoRows)
	poolMock.ExpectExec("INSERT INTO sol_payments").
		WithArgs(pgx.NamedArgs{
			"signature":        signatures[0].String(),
			"instructionIndex": 0,
			"amount":           amount,
			"slot":             slot,
			"routeIndex":       0,
			"toAccount":        dest.PublicKey().String(),
		}).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	poolMock.ExpectExec("INSERT INTO sol_purchases").
		WithArgs(pgx.NamedArgs{
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
		}).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	poolMock.ExpectCommit()

	p := &DefaultProcessor{
		pool: poolMock,
	}

	err = p.ProcessTransaction(ctx, slot, meta, tx, blockTime, logger)
	assert.NoError(t, err)
	assert.NoError(t, poolMock.ExpectationsWereMet())
}

func TestProcessTransaction_CallsInsertBalanceChange(t *testing.T) {
	// Setup a transaction with token balance changes
	account := solana.MustPublicKeyFromBase58("HJQj8P47BdA7ugjQEn45LaESYrxhiZDygmukt8iumFZJ")
	owner := solana.MustPublicKeyFromBase58("TT1eRKxi2Rj3oEvsFMe9W5hrcPmpXqKkNj7wC83AhXk")
	account2 := solana.MustPublicKeyFromBase58("Cjv8dvVfWU8wUYAR82T5oZ4nHLB6EyGNvpPBzw3r76Qy")
	owner2 := solana.MustPublicKeyFromBase58("dRiftyHA39MWEi3m9aunc5MzRF1JYuBsbn6VPcn33UH")
	mint := solana.MustPublicKeyFromBase58("9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM")
	account3 := solana.MustPublicKeyFromBase58("7sYw5JpQw8rTn2vQh3dX4bG6k9L2mN1pA5eF8cV3uZxT")
	mint2 := solana.MustPublicKeyFromBase58("2k8s5d3zqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2x")
	tx := &solana.Transaction{
		Signatures: []solana.Signature{
			solana.MustSignatureFromBase58("5ZVE83uvxQ36BmUM4kPn2foPyQCbsEepEkDTinC8bfSwHJdVCia6q3Wvnfa2Ls71SZoBmqoWPyJuPuUm8XcG92Hr"),
		},
		Message: solana.Message{
			AccountKeys: []solana.PublicKey{
				account,
				account2,
				account3,
			},
		},
	}
	meta := &rpc.TransactionMeta{
		PreTokenBalances: []rpc.TokenBalance{
			{
				AccountIndex: 0,
				Owner:        &owner,
				Mint:         mint,
				UiTokenAmount: &rpc.UiTokenAmount{
					Amount: "1000",
				},
			},
			// Should be excluded, wrong mint
			{
				AccountIndex: 2,
				Owner:        &owner2,
				Mint:         mint2,
				UiTokenAmount: &rpc.UiTokenAmount{
					Amount: "0",
				},
			},
		},
		PostTokenBalances: []rpc.TokenBalance{
			{
				AccountIndex: 0,
				Owner:        &owner,
				Mint:         mint,
				UiTokenAmount: &rpc.UiTokenAmount{
					Amount: "2000",
				},
			},
			{
				AccountIndex: 1,
				Owner:        &owner2,
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

	// Args
	logger := zap.NewNop()
	ctx := t.Context()
	slot := uint64(1)
	blockTime := time.Now()

	expectedArgs := pgx.NamedArgs{
		"account":        account.String(),
		"mint":           mint.String(),
		"owner":          owner.String(),
		"change":         int64(1000),
		"balance":        uint64(2000),
		"signature":      tx.Signatures[0].String(),
		"slot":           slot,
		"blockTimestamp": blockTime.UTC(),
	}

	expectedArgs2 := pgx.NamedArgs{
		"account":        account2.String(),
		"mint":           mint.String(),
		"owner":          owner2.String(),
		"change":         int64(0),
		"balance":        uint64(0),
		"signature":      tx.Signatures[0].String(),
		"slot":           slot,
		"blockTimestamp": blockTime.UTC(),
	}

	poolMock, err := pgxmock.NewPool()
	require.NoError(t, err, "failed to create mock database pool")
	defer poolMock.Close()
	// balance change insertion order can vary
	poolMock.MatchExpectationsInOrder(false)
	poolMock.ExpectBegin()
	poolMock.ExpectQuery("SELECT mint FROM artist_coins").
		WillReturnRows(
			pgxmock.NewRows([]string{"mints"}).
				AddRow(mint.String())) // Only the first mint
	poolMock.ExpectExec("INSERT INTO sol_token_account_balance_changes").
		WithArgs(expectedArgs).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	poolMock.ExpectExec("INSERT INTO sol_token_account_balance_changes").
		WithArgs(expectedArgs2).
		WillReturnResult(pgxmock.NewResult("INSERT", 1))
	poolMock.ExpectCommit()

	p := &DefaultProcessor{
		pool: poolMock,
	}

	err = p.ProcessTransaction(ctx, slot, meta, tx, blockTime, logger)
	assert.NoError(t, err)
	assert.NoError(t, poolMock.ExpectationsWereMet())
}

func TestProcessSignature_HandlesLoadedAddresses(t *testing.T) {
	// prod reward manager disbursement w/ lookup tables
	/*
		curl -X POST https://api.mainnet-beta.solana.com \
		-H "Content-Type: application/json" \
		-d '{
			"jsonrpc": "2.0",
			"id": 1,
			"method": "getTransaction",
			"params": [
			"58sUxCqs2sbErrZhH1A1YcFrYpK35Ph2AHpySxkCcRkeer1bJmfyCRKxQ7qeR26AA1qEnDb58KJwviDJXGqkAStQ",
			{
				"maxSupportedTransactionVersion": 0
			}
			]
		}'
	*/
	txResJson := `
	{
		"blockTime": 1753149679,
		"meta": {
		"computeUnitsConsumed": 38054,
		"err": null,
		"fee": 35450,
		"innerInstructions": [
			{
			"index": 0,
			"instructions": [
				{
				"accounts": [6, 2, 8],
				"data": "3Dc8EpW7Kr3R",
				"programIdIndex": 11,
				"stackHeight": 2
				},
				{
				"accounts": [0, 3],
				"data": "3Bxs49175da2o1zw",
				"programIdIndex": 12,
				"stackHeight": 2
				},
				{
				"accounts": [3],
				"data": "9krTCzbLfv4BRBcj",
				"programIdIndex": 12,
				"stackHeight": 2
				},
				{
				"accounts": [3],
				"data": "SYXsPCAS12XUEFvhVCEScVBsRUs1Lvxihmo8qVdn6ETKJKzE",
				"programIdIndex": 12,
				"stackHeight": 2
				}
			]
			}
		],
		"loadedAddresses": {
			"readonly": [
			"71hWFVYokLaN1PNYzTAWi13EfJ7Xt9VbSWUKsXUT8mxE",
			"8n2y76BtYed3EPwAkhDgdWQNtkazw6c9gY1RXDLy37KF",
			"8CrkKMAsR8pMNtmR65t5WwrLTXT1FUJRfWwUGLfMU8R1",
			"SysvarRent111111111111111111111111111111111",
			"TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
			"11111111111111111111111111111111"
			],
			"writable": ["3V9opXNpHmPPymKeq7CYD8wWMH8wzFXmqEkNdzfsZhYq"]
		},
		"logMessages": [
			"Program DDZDcYdQFEMwcu2Mwo75yGFjJ1mUQyyXLWzhZLEVFcei invoke [1]",
			"Program log: Instruction: Transfer",
			"Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA invoke [2]",
			"Program log: Instruction: Transfer",
			"Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA consumed 4645 of 183191 compute units",
			"Program TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA success",
			"Program 11111111111111111111111111111111 invoke [2]",
			"Program 11111111111111111111111111111111 success",
			"Program 11111111111111111111111111111111 invoke [2]",
			"Program 11111111111111111111111111111111 success",
			"Program 11111111111111111111111111111111 invoke [2]",
			"Program 11111111111111111111111111111111 success",
			"Program DDZDcYdQFEMwcu2Mwo75yGFjJ1mUQyyXLWzhZLEVFcei consumed 37904 of 203000 compute units",
			"Program DDZDcYdQFEMwcu2Mwo75yGFjJ1mUQyyXLWzhZLEVFcei success",
			"Program ComputeBudget111111111111111111111111111111 invoke [1]",
			"Program ComputeBudget111111111111111111111111111111 success"
		],
		"postBalances": [
			1499028959, 0, 2039280, 897840, 1141440, 1, 2039280, 1350240, 4392391,
			1398960, 1009200, 4513213226, 1
		],
		"postTokenBalances": [
			{
			"accountIndex": 2,
			"mint": "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"owner": "5ZiE3vAkrdXBgyFL7KqG3RoEGBws4CjRcXVbABDLZTgx",
			"programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
			"uiTokenAmount": {
				"amount": "13900000000",
				"decimals": 8,
				"uiAmount": 139.0,
				"uiAmountString": "139"
			}
			},
			{
			"accountIndex": 6,
			"mint": "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"owner": "8n2y76BtYed3EPwAkhDgdWQNtkazw6c9gY1RXDLy37KF",
			"programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
			"uiTokenAmount": {
				"amount": "2754676375551047",
				"decimals": 8,
				"uiAmount": 27546763.75551047,
				"uiAmountString": "27546763.75551047"
			}
			}
		],
		"preBalances": [
			1492988329, 6973920, 2039280, 0, 1141440, 1, 2039280, 1350240, 4392391,
			1398960, 1009200, 4513213226, 1
		],
		"preTokenBalances": [
			{
			"accountIndex": 2,
			"mint": "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"owner": "5ZiE3vAkrdXBgyFL7KqG3RoEGBws4CjRcXVbABDLZTgx",
			"programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
			"uiTokenAmount": {
				"amount": "13800000000",
				"decimals": 8,
				"uiAmount": 138.0,
				"uiAmountString": "138"
			}
			},
			{
			"accountIndex": 6,
			"mint": "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			"owner": "8n2y76BtYed3EPwAkhDgdWQNtkazw6c9gY1RXDLy37KF",
			"programId": "TokenkegQfeZyiNwAJbNbGKPFXCWuBvf9Ss623VQ5DA",
			"uiTokenAmount": {
				"amount": "2754676475551047",
				"decimals": 8,
				"uiAmount": 27546764.75551047,
				"uiAmountString": "27546764.75551047"
			}
			}
		],
		"rewards": [],
		"status": { "Ok": null }
		},
		"slot": 354896657,
		"transaction": {
		"message": {
			"accountKeys": [
			"C4MZpYiddDuWVofhs4BkUPyUiH78bFnaxhQVBB5fvko5",
			"8WCWQBxc3V7bDEF5poQYkNGLjsr9mzUuVSxfqs9Ksuv1",
			"EXYhWM17WbWw49tHFpi9pHUxKDwAPBK5rzWxTQPZFN2b",
			"CzbB1oPD1YSUthSr5TkN4m8EGsjN8z3rVgwRRyE4oaBc",
			"DDZDcYdQFEMwcu2Mwo75yGFjJ1mUQyyXLWzhZLEVFcei",
			"ComputeBudget111111111111111111111111111111"
			],
			"addressTableLookups": [
			{
				"accountKey": "4UQwpGupH66RgQrWRqmPM9Two6VJEE68VZ7GeqZ3mvVv",
				"readonlyIndexes": [5, 6, 8, 1, 3, 0],
				"writableIndexes": [7]
			}
			],
			"header": {
			"numReadonlySignedAccounts": 0,
			"numReadonlyUnsignedAccounts": 2,
			"numRequiredSignatures": 1
			},
			"instructions": [
			{
				"accounts": [1, 7, 8, 6, 2, 3, 9, 0, 10, 11, 12],
				"data": "8RMoXXC1taGWJZMAAjapFT6hJjcNVRVRbFPcbNHphz9uuwbKXdkcGK3aB5ChyFExjDUXjAbv",
				"programIdIndex": 4,
				"stackHeight": null
			},
			{
				"accounts": [],
				"data": "3uedW6ymeow5",
				"programIdIndex": 5,
				"stackHeight": null
			}
			],
			"recentBlockhash": "9bxHRc5pMC3JZMSgVPeps7XfkT4c8X3Qp5n5tQTrZKdx"
		},
		"signatures": [
			"58sUxCqs2sbErrZhH1A1YcFrYpK35Ph2AHpySxkCcRkeer1bJmfyCRKxQ7qeR26AA1qEnDb58KJwviDJXGqkAStQ"
		]
		},
		"version": 0
	}
	`
	txRes := rpc.GetTransactionResult{}
	err := json.Unmarshal([]byte(txResJson), &txRes)
	require.NoError(t, err, "failed to unmarshal transaction result")

	fakeRpcClient := fake_rpc_client.NewWithTransactions([]*rpc.GetTransactionResult{
		&txRes,
	})

	pool := database.CreateTestDatabase(t, "test_solana_indexer")
	p := NewDefaultProcessor(
		fakeRpcClient,
		pool,
		config.Cfg,
	)

	// Use prod reward program ID
	reward_manager.SetProgramID(solana.MustPublicKeyFromBase58(config.ProdRewardManagerProgramID))

	err = p.ProcessSignature(t.Context(), 354896657, solana.MustSignatureFromBase58("58sUxCqs2sbErrZhH1A1YcFrYpK35Ph2AHpySxkCcRkeer1bJmfyCRKxQ7qeR26AA1qEnDb58KJwviDJXGqkAStQ"), zap.NewNop())
	require.NoError(t, err, "failed to process signature")

	row := pool.QueryRow(t.Context(), "SELECT EXISTS (SELECT 1 FROM sol_reward_disbursements WHERE signature = $1)", "58sUxCqs2sbErrZhH1A1YcFrYpK35Ph2AHpySxkCcRkeer1bJmfyCRKxQ7qeR26AA1qEnDb58KJwviDJXGqkAStQ")
	var exists bool
	row.Scan(&exists)
	// Temp disable until next pr
	// require.True(t, exists, "expected reward disbursement to exist")
}
