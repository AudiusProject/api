package api

import (
	"testing"

	"api.audius.co/database"
)

func TestMetricsTotalWallets_Empty(t *testing.T) {
	app := emptyTestApp(t)

	status, body := testGet(t, app, "/v1/metrics/total_wallets")
	if status != 200 {
		t.Fatalf("expected 200, got %d: %s", status, string(body))
	}

	jsonAssert(t, body, map[string]any{
		"data.total": 0,
	})
}

func TestMetricsTotalWallets_WithFixtures(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "handle": "u1"},
			{"user_id": 2, "handle": "u2"},
			{"user_id": 3, "handle": "u3"},
			{"user_id": 4, "handle": "u4"},
		},
		"sol_claimable_accounts": {
			{"signature": "sig1", "instruction_index": 0, "slot": 1, "mint": "mint1", "ethereum_address": "0xabc", "account": "acc1"},
			{"signature": "sig2", "instruction_index": 0, "slot": 1, "mint": "mint2", "ethereum_address": "0xdef", "account": "acc2"},
			{"signature": "sig3", "instruction_index": 0, "slot": 1, "mint": "mint3", "ethereum_address": "0xghi", "account": "acc3"},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/metrics/total_wallets")
	if status != 200 {
		t.Fatalf("expected 200, got %d: %s", status, string(body))
	}

	jsonAssert(t, body, map[string]any{
		"data.total": 7,
	})
}
