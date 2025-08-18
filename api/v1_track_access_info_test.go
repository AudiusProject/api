package api

import (
	"testing"

	"bridgerton.audius.co/config"
	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1TrackAccessInfo(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id":        1001,
				"owner_id":        175425,
				"title":           "Gated Track",
				"is_stream_gated": true,
				"stream_conditions": map[string]any{
					"usdc_purchase": map[string]any{
						"price": 100.0,
						"splits": []map[string]any{
							{
								"user_id":    175425,
								"percentage": 100.0,
							},
						},
					},
				},
				"is_download_gated": true,
				"download_conditions": map[string]any{
					"usdc_purchase": map[string]any{
						"price": 100.0,
						"splits": []map[string]any{
							{
								"user_id":    175425,
								"percentage": 100.0,
							},
						},
					},
				},
			},
			{
				"track_id":          1002,
				"owner_id":          175425,
				"title":             "Free Track",
				"is_stream_gated":   false,
				"is_download_gated": false,
			},
		},
		"users": []map[string]any{
			{
				"user_id":   175425,
				"handle":    "testartist",
				"handle_lc": "testartist",
				"wallet":    "0xd4302f79457d5f5fcd54afd9e5a1a399723e7c30",
				// Note that this address intentionally starts with a z to test that
				// sorting is correct (split with userid shows up first)
				"spl_usdc_payout_wallet": "zRkzQYez2yMUmfe3kdWUjNhCXmw6xgReovBCsyVGi5E",
			},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	{
		status, body := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1001)+"/access-info")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			// Basic track info
			"data.access.stream":     false,
			"data.access.download":   false,
			"data.user_id":           trashid.MustEncodeHashID(175425),
			"data.blocknumber":       101,
			"data.is_stream_gated":   true,
			"data.is_download_gated": true,

			// Stream conditions
			"data.stream_conditions.usdc_purchase.price": 100.0,

			// User split in stream conditions
			"data.stream_conditions.usdc_purchase.splits.0.user_id":       175425,
			"data.stream_conditions.usdc_purchase.splits.0.percentage":    100.0,
			"data.stream_conditions.usdc_purchase.splits.0.amount":        900000,
			"data.stream_conditions.usdc_purchase.splits.0.eth_wallet":    "0xd4302f79457d5f5fcd54afd9e5a1a399723e7c30",
			"data.stream_conditions.usdc_purchase.splits.0.payout_wallet": "zRkzQYez2yMUmfe3kdWUjNhCXmw6xgReovBCsyVGi5E",

			// Network cut in stream conditions
			"data.stream_conditions.usdc_purchase.splits.1.user_id":       nil,
			"data.stream_conditions.usdc_purchase.splits.1.percentage":    10.0,
			"data.stream_conditions.usdc_purchase.splits.1.amount":        100000,
			"data.stream_conditions.usdc_purchase.splits.1.eth_wallet":    nil,
			"data.stream_conditions.usdc_purchase.splits.1.payout_wallet": config.Cfg.SolanaConfig.StakingBridgeUsdcTokenAccount.String(),

			// Download conditions (should match stream conditions)
			"data.download_conditions.usdc_purchase.price": 100.0,

			// User split in download conditions
			"data.download_conditions.usdc_purchase.splits.0.user_id":       175425,
			"data.download_conditions.usdc_purchase.splits.0.percentage":    100.0,
			"data.download_conditions.usdc_purchase.splits.0.amount":        900000,
			"data.download_conditions.usdc_purchase.splits.0.eth_wallet":    "0xd4302f79457d5f5fcd54afd9e5a1a399723e7c30",
			"data.download_conditions.usdc_purchase.splits.0.payout_wallet": "zRkzQYez2yMUmfe3kdWUjNhCXmw6xgReovBCsyVGi5E",

			// Network cut in download conditions
			"data.download_conditions.usdc_purchase.splits.1.user_id":       nil,
			"data.download_conditions.usdc_purchase.splits.1.percentage":    10.0,
			"data.download_conditions.usdc_purchase.splits.1.amount":        100000,
			"data.download_conditions.usdc_purchase.splits.1.eth_wallet":    nil,
			"data.download_conditions.usdc_purchase.splits.1.payout_wallet": config.Cfg.SolanaConfig.StakingBridgeUsdcTokenAccount.String(),
		})
	}

	// Test free track access info
	{
		status, body := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1002)+"/access-info")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			// Basic track info
			"data.access.stream":     true, // Free track should be streamable
			"data.access.download":   true, // Free track should be downloadable
			"data.user_id":           trashid.MustEncodeHashID(175425),
			"data.blocknumber":       101,
			"data.is_stream_gated":   false,
			"data.is_download_gated": false,

			// No gating conditions
			"data.stream_conditions":   nil,
			"data.download_conditions": nil,
		})
	}

	// Test non-existent track
	{
		status, _ := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(9999)+"/access-info")
		assert.Equal(t, 404, status)
	}
}
