package api

import (
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestDashboardWalletUsers(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id":   1,
				"handle":    "testuser",
				"handle_lc": "testuser",
				"wallet":    "0x1234567890abcdef",
			},
			{
				"user_id":   2,
				"handle":    "anotheruser",
				"handle_lc": "anotheruser",
				"wallet":    "0xabcdef1234567890",
			},
		},
		"dashboard_wallet_users": {
			{
				"wallet":  "0xTEST123WALLET",
				"user_id": 1,
				"txhash":  "testhash1",
			},
			{
				"wallet":  "0xANOTHERWALLET",
				"user_id": 2,
				"txhash":  "testhash2",
			},
			{
				"wallet":    "0xDELETEDWALLET",
				"user_id":   2,
				"is_delete": true,
				"txhash":    "testhash3",
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	var resp struct {
		Data []MinDashboardWalletUser
	}

	status, body := testGet(t, app, "/v1/dashboard_wallet_users?wallets=0xtest123wallet,0xanotherwallet,0xdeletedwallet,0xnonexistent", &resp)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":             2,
		"data.0.wallet":      "0xTEST123WALLET",
		"data.0.user.handle": "testuser",
		"data.0.user.id":     trashid.MustEncodeHashID(1),
		"data.1.wallet":      "0xANOTHERWALLET",
		"data.1.user.handle": "anotheruser",
		"data.1.user.id":     trashid.MustEncodeHashID(2),
	})
}
