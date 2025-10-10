package api

import (
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

const user1Wallet = "0x7d273271690538cf855e5b3002a0dd8c154bb060"

func TestV1UsersSalesDownloadJson(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "seller", "wallet": user1Wallet},
			{"user_id": 2, "handle": "buyer1", "name": "buyer1"},
			{"user_id": 3, "handle": "buyer2", "name": "buyer2"},
		},
		"tracks": []map[string]any{
			{"track_id": 1, "title": "track1", "owner_id": 1},
		},
		"track_routes": []map[string]any{
			{
				"track_id":     1,
				"owner_id":     1,
				"slug":         "track1",
				"title_slug":   "track1",
				"collision_id": 0,
			},
		},
		"usdc_purchases": []map[string]any{
			{
				"seller_user_id": 1,
				"buyer_user_id":  2,
				"content_type":   "track",
				"content_id":     1,
				"amount":         1000000,
				"extra_amount":   0,
				"created_at":     "2024-06-04 00:00:00",
				"signature":      "def",
				"splits": []map[string]any{
					{"user_id": 2, "payout_wallet": "buyer1wallet", "amount": 1000000},
					{"payout_wallet": "testUSDCStakingBridge", "amount": 100000, "percentage": 10},
				},
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	{
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/sales/download/json", user1Wallet)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.sales.#": 1})
	}
}
