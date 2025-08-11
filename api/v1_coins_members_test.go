package api

import (
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1CoinsMembers(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"artist_coins": {
			{
				"ticker":   "$ARTISTCOIN",
				"decimals": 1,
				"user_id":  1,
				"mint":     "artistcoin_mint",
			},
		},
		"users": {
			{
				"user_id": 1,
				"wallet":  "0x123456789012345678901234567890",
			},
			{
				"user_id": 2,
				"wallet":  "0x098765432109876543210987654321",
			},
			{
				"user_id": 3,
				"wallet":  "0x112233445566778899001122334455",
			},
		},
		"associated_wallets": {
			{
				"id":      1,
				"user_id": 1,
				"wallet":  "owner_wallet1-1",
				"chain":   "sol",
			},
			{
				"id":      4,
				"user_id": 1,
				"wallet":  "owner_wallet1-2",
				"chain":   "sol",
			},
			{
				"id":      5,
				"user_id": 1,
				"wallet":  "owner_wallet1-3",
				"chain":   "sol",
			},
			{
				"id":      2,
				"user_id": 2,
				"wallet":  "owner_wallet2",
				"chain":   "sol",
			},
			{
				"id":      3,
				"user_id": 3,
				"wallet":  "owner_wallet3",
				"chain":   "sol",
			},
		},
		"sol_claimable_accounts": {
			{
				"signature":        "claimable_signature_1",
				"account":          "claimable_account1",
				"ethereum_address": "0x123456789012345678901234567890",
				"mint":             "artistcoin_mint",
			},
			{
				"signature":        "claimable_signature_2",
				"account":          "claimable_account2",
				"ethereum_address": "0x098765432109876543210987654321",
				"mint":             "artistcoin_mint",
			},
		},
		"sol_token_account_balances": {
			{
				"mint":    "artistcoin_mint",
				"account": "account1",
				"owner":   "owner_wallet1-1",
				"balance": 10,
			},
			{
				"mint":    "artistcoin_mint",
				"account": "account2",
				"owner":   "owner_wallet1-2",
				"balance": 20,
			},
			{
				"mint":    "artistcoin_mint",
				"account": "account3",
				"owner":   "owner_wallet1-3",
				"balance": 30,
			},
			{
				"mint":    "artistcoin_mint",
				"account": "account4",
				"owner":   "owner_wallet2",
				"balance": 41,
			},
			{
				"mint":    "artistcoin_mint",
				"account": "account5",
				"owner":   "owner_wallet3",
				"balance": 53,
			},
			{
				"mint":    "artistcoin_mint",
				"account": "claimable_account1",
				"balance": 60,
			},
			{
				"mint":    "artistcoin_mint",
				"account": "claimable_account2",
				"balance": 61,
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// Test w/o params
	{
		status, body := testGet(
			t, app,
			"/v1/coins/artistcoin_mint/members",
		)

		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":         3,
			"data.0.user_id": trashid.MustEncodeHashID(1),
			"data.0.balance": 120,
			"data.1.user_id": trashid.MustEncodeHashID(2),
			"data.1.balance": 102,
			"data.2.user_id": trashid.MustEncodeHashID(3),
			"data.2.balance": 53,
		})
	}

	// Test limit/offset
	{
		status, body := testGet(
			t, app,
			"/v1/coins/artistcoin_mint/members?limit=2&offset=1",
		)

		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":         2,
			"data.0.user_id": trashid.MustEncodeHashID(2),
			"data.0.balance": 102,
			"data.1.user_id": trashid.MustEncodeHashID(3),
			"data.1.balance": 53,
		})
	}

	// Test sort direction
	{
		status, body := testGet(
			t, app,
			"/v1/coins/artistcoin_mint/members?sort_direction=asc",
		)

		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":         3,
			"data.0.user_id": trashid.MustEncodeHashID(3),
			"data.0.balance": 53,
			"data.1.user_id": trashid.MustEncodeHashID(2),
			"data.1.balance": 102,
			"data.2.user_id": trashid.MustEncodeHashID(1),
			"data.2.balance": 120,
		})
	}

	// Test min balance
	{
		status, body := testGet(
			t, app,
			"/v1/coins/artistcoin_mint/members?min_balance=100&sort_direction=asc",
		)

		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":         2,
			"data.0.user_id": trashid.MustEncodeHashID(2),
			"data.0.balance": 102,
			"data.1.user_id": trashid.MustEncodeHashID(1),
			"data.1.balance": 120,
		})
	}
}
