package api

import (
	"context"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Tests for migration 0205_canonicalize_associated_wallets_eth.sql. The
// migration itself ran at test-DB setup (via make test-schema); these
// confirm the BEFORE INSERT/UPDATE trigger + that the migration's
// canonicalization step would have done the right thing on the data it
// would have seen on prod.

func TestCanonicalizeAssociatedWallets_TriggerLowercasesEth(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()

	const mixed = "0xAbCdEf1234567890abCDef1234567890ABCdEf12"
	const lower = "0xabcdef1234567890abcdef1234567890abcdef12"

	database.Seed(app.writePool, database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "u1", "wallet": "0xu1"},
		},
	})

	_, err := app.writePool.Exec(ctx, `
		INSERT INTO associated_wallets (id, user_id, wallet, blockhash, blocknumber, is_current, is_delete, chain)
		VALUES (100, 1, $1, 'h', 101, true, false, 'eth')
	`, mixed)
	require.NoError(t, err)

	var stored string
	err = app.writePool.QueryRow(ctx, `SELECT wallet FROM associated_wallets WHERE id = 100`).Scan(&stored)
	require.NoError(t, err)
	assert.Equal(t, lower, stored, "trigger must lowercase eth-chain wallets on insert")
}

// Trigger must NOT touch Solana wallets — base58 is case-sensitive.
func TestCanonicalizeAssociatedWallets_TriggerLeavesSolAlone(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()

	const solanaAddr = "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM" // mixed case, valid base58

	database.Seed(app.writePool, database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "u1", "wallet": "0xu1"},
		},
	})

	_, err := app.writePool.Exec(ctx, `
		INSERT INTO associated_wallets (id, user_id, wallet, blockhash, blocknumber, is_current, is_delete, chain)
		VALUES (101, 1, $1, 'h', 101, true, false, 'sol')
	`, solanaAddr)
	require.NoError(t, err)

	var stored string
	err = app.writePool.QueryRow(ctx, `SELECT wallet FROM associated_wallets WHERE id = 101`).Scan(&stored)
	require.NoError(t, err)
	assert.Equal(t, solanaAddr, stored, "trigger must leave sol wallets in their original case")
}

// Updates to eth wallets are also canonicalized.
func TestCanonicalizeAssociatedWallets_TriggerLowercasesOnUpdate(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()

	database.Seed(app.writePool, database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "u1", "wallet": "0xu1"},
		},
	})

	_, err := app.writePool.Exec(ctx, `
		INSERT INTO associated_wallets (id, user_id, wallet, blockhash, blocknumber, is_current, is_delete, chain)
		VALUES (102, 1, '0xstart', 'h', 101, true, false, 'eth')
	`)
	require.NoError(t, err)

	_, err = app.writePool.Exec(ctx, `
		UPDATE associated_wallets SET wallet = '0xMiXeDcAsE1234567890aBcDeF1234567890ABCDEF' WHERE id = 102
	`)
	require.NoError(t, err)

	var stored string
	err = app.writePool.QueryRow(ctx, `SELECT wallet FROM associated_wallets WHERE id = 102`).Scan(&stored)
	require.NoError(t, err)
	assert.Equal(t, "0xmixedcase1234567890abcdef1234567890abcdef", stored)
}
