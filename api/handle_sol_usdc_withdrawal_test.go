package api

import (
	"context"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Exercises the handle_sol_usdc_withdrawal trigger end-to-end: insert into
// sol_claimable_account_transfers (with the indexer's ordering so the
// memo marker is already present) and check that a notification row pops
// out with the right type + group_id. Mirrors the legacy
// handle_usdc_withdrawal trigger contract.
func TestHandleSolUsdcWithdrawalTrigger(t *testing.T) {
	const usdcMintAddr = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	app := emptyTestApp(t)
	ctx := context.Background()

	userWallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	userBank := "User1UsdcBank_trigger_test__________________"
	external := "0xextdest0000000000000000000000000000000000"

	// Seed explicitly (not via Seed()), because the trigger fires on
	// sol_claimable_account_transfers and reads sol_transfer_memo_types —
	// the memo marker must be present at trigger time. Mirrors the order the
	// indexer uses in claimable_tokens.go.
	_, err := app.writePool.Exec(ctx, `
		INSERT INTO public.blocks (blockhash, parenthash, is_current, number)
		VALUES ('block1', 'block0', true, 101)
		ON CONFLICT DO NOTHING;`)
	require.NoError(t, err)
	database.SeedTable(app.writePool, "users", []map[string]any{
		{"user_id": 1, "handle": "u", "wallet": userWallet, "is_current": true},
	})
	database.SeedTable(app.writePool, "sol_claimable_accounts", []map[string]any{
		{
			"signature":         "create_sig",
			"instruction_index": 0,
			"slot":              1,
			"mint":              usdcMintAddr,
			"ethereum_address":  userWallet,
			"account":           userBank,
		},
	})
	database.SeedTable(app.writePool, "sol_token_account_balance_changes", []map[string]any{
		{
			"signature":       "withdraw_sig",
			"mint":            usdcMintAddr,
			"owner":           "claimable-tokens-pda",
			"account":         userBank,
			"change":          -1500000,
			"balance":         500000,
			"slot":            100,
			"block_timestamp": time.Date(2024, 7, 1, 0, 0, 0, 0, time.UTC),
		},
		// Second flow: plain transfer (no memo) to assert that branch.
		{
			"signature":       "xfer_sig",
			"mint":            usdcMintAddr,
			"owner":           "claimable-tokens-pda",
			"account":         userBank,
			"change":          -250000,
			"balance":         250000,
			"slot":            200,
			"block_timestamp": time.Date(2024, 7, 2, 0, 0, 0, 0, time.UTC),
		},
	})
	// Memo marker BEFORE the claimable transfer (real-world ordering).
	database.SeedTable(app.writePool, "sol_transfer_memo_types", []map[string]any{
		{
			"signature":         "withdraw_sig",
			"instruction_index": 1,
			"slot":              100,
			"memo_type":         "withdrawal",
		},
	})
	// Trigger fires on these inserts.
	database.SeedTable(app.writePool, "sol_claimable_account_transfers", []map[string]any{
		{
			"signature":          "withdraw_sig",
			"instruction_index":  1,
			"amount":             1500000,
			"slot":               100,
			"from_account":       userBank,
			"to_account":         external,
			"sender_eth_address": userWallet,
		},
		{
			"signature":          "xfer_sig",
			"instruction_index":  1,
			"amount":             250000,
			"slot":               200,
			"from_account":       userBank,
			"to_account":         external,
			"sender_eth_address": userWallet,
		},
	})

	// Withdrawal row → usdc_withdrawal notification with the correct payload.
	var (
		notifType   string
		groupId     string
		userIds     []int32
		userBankOut string
		signature   string
		change      int64
		balance     int64
		receiver    string
	)
	err = app.writePool.QueryRow(ctx, `
		SELECT type, group_id, user_ids,
		       data->>'user_bank',
		       data->>'signature',
		       (data->>'change')::bigint,
		       (data->>'balance')::bigint,
		       data->>'receiver_account'
		  FROM notification
		 WHERE data->>'signature' = 'withdraw_sig'
	`).Scan(&notifType, &groupId, &userIds, &userBankOut, &signature, &change, &balance, &receiver)
	require.NoError(t, err)
	assert.Equal(t, "usdc_withdrawal", notifType)
	assert.Equal(t, "usdc_withdrawal:1:signature:withdraw_sig", groupId)
	assert.Equal(t, []int32{1}, userIds)
	assert.Equal(t, userBank, userBankOut)
	assert.Equal(t, "withdraw_sig", signature)
	assert.Equal(t, int64(-1500000), change)
	assert.Equal(t, int64(500000), balance)
	assert.Equal(t, external, receiver)

	// Plain transfer (no memo) → usdc_transfer notification.
	err = app.writePool.QueryRow(ctx, `
		SELECT type, group_id
		  FROM notification
		 WHERE data->>'signature' = 'xfer_sig'
	`).Scan(&notifType, &groupId)
	require.NoError(t, err)
	assert.Equal(t, "usdc_transfer", notifType)
	assert.Equal(t, "usdc_transfer:1:signature:xfer_sig", groupId)
}

// Sanity check: prepare_withdrawal / internal_transfer don't fire the
// notification (legacy trigger didn't notify on those types either).
func TestHandleSolUsdcWithdrawalTrigger_SkipsSystemTypes(t *testing.T) {
	const usdcMintAddr = "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v"
	app := emptyTestApp(t)
	ctx := context.Background()

	userWallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	userBank := "User1UsdcBank_skip_test_____________________"

	_, err := app.writePool.Exec(ctx, `
		INSERT INTO public.blocks (blockhash, parenthash, is_current, number)
		VALUES ('block1', 'block0', true, 101)
		ON CONFLICT DO NOTHING;`)
	require.NoError(t, err)
	database.SeedTable(app.writePool, "users", []map[string]any{
		{"user_id": 1, "handle": "u", "wallet": userWallet, "is_current": true},
	})
	database.SeedTable(app.writePool, "sol_claimable_accounts", []map[string]any{
		{"signature": "create_sig", "instruction_index": 0, "slot": 1, "mint": usdcMintAddr, "ethereum_address": userWallet, "account": userBank},
	})
	database.SeedTable(app.writePool, "sol_token_account_balance_changes", []map[string]any{
		{"signature": "prep_sig", "mint": usdcMintAddr, "owner": "p", "account": userBank, "change": -1000, "balance": 0, "slot": 10, "block_timestamp": time.Now()},
	})
	database.SeedTable(app.writePool, "sol_transfer_memo_types", []map[string]any{
		{"signature": "prep_sig", "instruction_index": 1, "slot": 10, "memo_type": "prepare_withdrawal"},
	})
	database.SeedTable(app.writePool, "sol_claimable_account_transfers", []map[string]any{
		{"signature": "prep_sig", "instruction_index": 1, "amount": 1000, "slot": 10, "from_account": userBank, "to_account": "jupiter", "sender_eth_address": userWallet},
	})

	var count int
	err = app.writePool.QueryRow(ctx, `
		SELECT count(*) FROM notification WHERE data->>'signature' = 'prep_sig'
	`).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count, "prepare_withdrawal should not produce a usdc_* notification")
}
