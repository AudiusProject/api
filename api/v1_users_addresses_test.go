package api

import (
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UserIdsByAddresses(t *testing.T) {
	app := emptyTestApp(t)

	// Seed with users, associated_wallets, and sol_claimable_accounts
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "wallet": "0xabc"},
			{"user_id": 2, "wallet": "0xdef"},
		},
		"associated_wallets": []map[string]any{
			{"id": 1, "user_id": 1, "wallet": "0xaaa", "chain": "sol"},
			{"id": 2, "user_id": 2, "wallet": "0xbbb", "chain": "sol"},
		},
		"sol_claimable_accounts": []map[string]any{
			{"account": "sol_acc_1", "ethereum_address": "0xabc", "mint": "mint1", "signature": "sig1"},
			{"account": "sol_acc_2", "ethereum_address": "0xdef", "mint": "mint2", "signature": "sig2"},
		},
	}
	database.Seed(app.pool, fixtures)

	{
		// Query for all types of addresses
		status, body := testGet(t, app, "/v1/users/address?address=0xabc&address=0xaaa&address=sol_acc_1")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":         3,
			"data.0.user_id": trashid.MustEncodeHashID(1),
			"data.1.user_id": trashid.MustEncodeHashID(1),
			"data.2.user_id": trashid.MustEncodeHashID(1),
		})
	}

	{
		// Query a single address
		status, body := testGet(t, app, "/v1/users/address?address=0xbbb")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":         1,
			"data.0.user_id": trashid.MustEncodeHashID(2),
		})
	}
}
