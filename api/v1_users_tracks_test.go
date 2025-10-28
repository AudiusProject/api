package api

import (
	"fmt"
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetUserTracks(t *testing.T) {
	app := testAppWithFixtures(t)

	var userTracksResponse struct {
		Data []dbv1.FullTrack
	}

	// Test support for handle
	status, body := testGet(t, app, "/v1/full/users/handle/usertrackstester/tracks", &userTracksResponse)

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(703),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Remaining assertions use the user_id version of the route
	baseUrl := fmt.Sprintf("/v1/full/users/%s/tracks", trashid.MustEncodeHashID(500))

	status, body = testGet(t, app, baseUrl, &userTracksResponse)
	assert.Equal(t, 200, status)

	// Note: Date sorts prefer release_date but fall back to created_at
	// Default sort by legacy date desc - artist pick (701) should be first
	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(703),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Sort by date asc - artist pick (701) should be first
	url := fmt.Sprintf("%s?sort=date&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(700),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Release date desc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=release_date&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(703),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Release date asc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=release_date&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(700),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Sort by legacy plays desc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort=plays&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(700),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Sort by legacy plays asc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort=plays&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(703),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Sort by sort_method plays desc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=plays&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(700),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Sort by sort_method plays asc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=plays&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(703),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Sort by title desc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=title&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first (title: Track 4)
		"data.1.id": trashid.MustEncodeHashID(702), // Track 3
		"data.2.id": trashid.MustEncodeHashID(703), // Track 2
		"data.3.id": trashid.MustEncodeHashID(700), // Track 1
	})

	// Sort by title asc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=title&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first (title: Track 4)
		"data.1.id": trashid.MustEncodeHashID(700), // Track 1
		"data.2.id": trashid.MustEncodeHashID(703), // Track 2
		"data.3.id": trashid.MustEncodeHashID(702), // Track 3
	})

	// Sort by reposts desc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=reposts&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first (75 reposts)
		"data.1.id": trashid.MustEncodeHashID(702), // 100 reposts
		"data.2.id": trashid.MustEncodeHashID(700), // 50 reposts
		"data.3.id": trashid.MustEncodeHashID(703), // 25 reposts
	})

	// Sort by reposts asc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=reposts&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first (75 reposts)
		"data.1.id": trashid.MustEncodeHashID(703), // 25 reposts
		"data.2.id": trashid.MustEncodeHashID(700), // 50 reposts
		"data.3.id": trashid.MustEncodeHashID(702), // 100 reposts
	})

	// Sort by saves desc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=saves&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first (100 saves)
		"data.1.id": trashid.MustEncodeHashID(702), // 75 saves
		"data.2.id": trashid.MustEncodeHashID(700), // 50 saves
		"data.3.id": trashid.MustEncodeHashID(703), // 25 saves
	})

	// Sort by saves asc
	url = fmt.Sprintf("%s?sort_method=saves&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(703),
		"data.2.id": trashid.MustEncodeHashID(700),
		"data.3.id": trashid.MustEncodeHashID(702),
	})

	// Test artist pick is always first regardless of sort
	// Artist pick should be first even with title desc sort
	url = fmt.Sprintf("%s?sort_method=title&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first (track 4)
	})

	// Artist pick should be first even with plays desc sort
	url = fmt.Sprintf("%s?sort=plays&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
	})

}

func TestGetUserTracksInvalidParams(t *testing.T) {
	app := testAppWithFixtures(t)
	baseUrl := fmt.Sprintf("/v1/full/users/%s/tracks", trashid.MustEncodeHashID(500))
	// Test invalid sort_method
	url := fmt.Sprintf("%s?sort_method=invalid&sort_direction=desc", baseUrl)
	status, _ := testGet(t, app, url)
	assert.Equal(t, 400, status)

	// Test invalid sort_direction
	url = fmt.Sprintf("%s?sort_method=plays&sort_direction=invalid", baseUrl)
	status, _ = testGet(t, app, url)
	assert.Equal(t, 400, status)

	// Test invalid sort
	url = fmt.Sprintf("%s?sort=invalid", baseUrl)
	status, _ = testGet(t, app, url)
	assert.Equal(t, 400, status)

	// Test invalid limit
	url = fmt.Sprintf("%s?limit=101", baseUrl)
	status, _ = testGet(t, app, url)
	assert.Equal(t, 400, status)

	// Test invalid offset
	url = fmt.Sprintf("%s?offset=invalid", baseUrl)
	status, _ = testGet(t, app, url)
	assert.Equal(t, 400, status)
}

func TestGetUserTracksWithGateConditionFilter(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := testTrackGateFixtures()
	database.Seed(app.pool.Replicas[0], fixtures)

	var userTracksResponse struct {
		Data []dbv1.FullTrack
	}

	baseUrl := fmt.Sprintf("/v1/full/users/%s/tracks", trashid.MustEncodeHashID(600))

	// Test without filter - should return all tracks
	status, _ := testGet(t, app, baseUrl, &userTracksResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 6, len(userTracksResponse.Data))

	// Test filter for ungated tracks only
	url := fmt.Sprintf("%s?gate_condition=ungated", baseUrl)
	status, _ = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(userTracksResponse.Data))
	assert.Equal(t, trashid.MustEncodeHashID(800), userTracksResponse.Data[0].ID)

	// Test filter for usdc_purchase gated tracks
	url = fmt.Sprintf("%s?gate_condition=usdc_purchase", baseUrl)
	status, _ = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(userTracksResponse.Data))
	assert.Equal(t, trashid.MustEncodeHashID(801), userTracksResponse.Data[0].ID)

	// Test filter for follow gated tracks
	url = fmt.Sprintf("%s?gate_condition=follow", baseUrl)
	status, _ = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(userTracksResponse.Data))
	assert.Equal(t, trashid.MustEncodeHashID(802), userTracksResponse.Data[0].ID)

	// Test filter for tip gated tracks
	url = fmt.Sprintf("%s?gate_condition=tip", baseUrl)
	status, _ = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(userTracksResponse.Data))
	assert.Equal(t, trashid.MustEncodeHashID(803), userTracksResponse.Data[0].ID)

	// Test filter for nft gated tracks
	url = fmt.Sprintf("%s?gate_condition=nft", baseUrl)
	status, _ = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(userTracksResponse.Data))
	assert.Equal(t, trashid.MustEncodeHashID(804), userTracksResponse.Data[0].ID)

	// Test filter for token gated tracks
	url = fmt.Sprintf("%s?gate_condition=token", baseUrl)
	status, _ = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(userTracksResponse.Data))
	assert.Equal(t, trashid.MustEncodeHashID(805), userTracksResponse.Data[0].ID)

	// Test multiple gate conditions (usdc_purchase OR tip)
	url = fmt.Sprintf("%s?gate_condition=usdc_purchase&gate_condition=tip", baseUrl)
	status, _ = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(userTracksResponse.Data))

	// Test multiple gate conditions (ungated OR follow OR nft)
	url = fmt.Sprintf("%s?gate_condition=ungated&gate_condition=follow&gate_condition=nft", baseUrl)
	status, _ = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)
	assert.Equal(t, 3, len(userTracksResponse.Data))
}

func testTrackGateFixtures() map[string][]map[string]any {
	return map[string][]map[string]any{
		"users": {
			{
				"user_id":   600,
				"handle":    "gatedtrackstester",
				"handle_lc": "gatedtrackstester",
				"wallet":    "0xd4302f79457d5f5fcd54afd9e5a1a399723e7c30",
			},
		},
		"tracks": {
			{
				"track_id":        800,
				"owner_id":        600,
				"title":           "Ungated Track",
				"is_stream_gated": false,
				"created_at":      "2021-01-01 00:00:00",
			},
			{
				"track_id":        801,
				"owner_id":        600,
				"title":           "USDC Purchase Gated Track",
				"is_stream_gated": true,
				"stream_conditions": map[string]any{
					"usdc_purchase": map[string]any{
						"price": 100.0,
						"splits": []map[string]any{
							{
								"user_id":    600,
								"percentage": 100.0,
							},
						},
					},
				},
				"created_at": "2021-01-02 00:00:00",
			},
			{
				"track_id":          802,
				"owner_id":          600,
				"title":             "Follow Gated Track",
				"is_stream_gated":   true,
				"stream_conditions": map[string]any{"follow_user_id": 600},
				"created_at":        "2021-01-03 00:00:00",
			},
			{
				"track_id":          803,
				"owner_id":          600,
				"title":             "Tip Gated Track",
				"is_stream_gated":   true,
				"stream_conditions": map[string]any{"tip_user_id": 600},
				"created_at":        "2021-01-04 00:00:00",
			},
			{
				"track_id":        804,
				"owner_id":        600,
				"title":           "NFT Gated Track",
				"is_stream_gated": true,
				"stream_conditions": map[string]any{
					"nft_collection": map[string]any{
						"chain":   "eth",
						"address": "0x1234567890123456789012345678901234567890",
					},
				},
				"created_at": "2021-01-05 00:00:00",
			},
			{
				"track_id":        805,
				"owner_id":        600,
				"title":           "Token Gated Track",
				"is_stream_gated": true,
				"stream_conditions": map[string]any{
					"token_gate": map[string]any{
						"token_mint":   "7i5KKsX2weiTkry7jA4ZwSuXGhs5eJBEjY8vVxR4pfRx",
						"token_amount": 100,
					},
				},
				"created_at": "2021-01-06 00:00:00",
			},
		},
	}
}

func TestBuildGateConditionFilter(t *testing.T) {
	// Test with no conditions
	result := buildGateConditionFilter([]string{})
	assert.Equal(t, "", result)

	// Test with single ungated condition
	result = buildGateConditionFilter([]string{"ungated"})
	assert.Contains(t, result, "t.is_stream_gated = false")

	// Test with single usdc_purchase condition
	result = buildGateConditionFilter([]string{"usdc_purchase"})
	assert.Contains(t, result, "t.is_stream_gated = true")
	assert.Contains(t, result, "t.stream_conditions->>'usdc_purchase' IS NOT NULL")

	// Test with single follow condition
	result = buildGateConditionFilter([]string{"follow"})
	assert.Contains(t, result, "t.stream_conditions->>'follow_user_id' IS NOT NULL")

	// Test with single tip condition
	result = buildGateConditionFilter([]string{"tip"})
	assert.Contains(t, result, "t.stream_conditions->>'tip_user_id' IS NOT NULL")

	// Test with single nft condition
	result = buildGateConditionFilter([]string{"nft"})
	assert.Contains(t, result, "t.stream_conditions->>'nft_collection' IS NOT NULL")

	// Test with single token condition
	result = buildGateConditionFilter([]string{"token"})
	assert.Contains(t, result, "t.stream_conditions->>'token_gate' IS NOT NULL")

	// Test with multiple conditions
	result = buildGateConditionFilter([]string{"ungated", "usdc_purchase"})
	assert.Contains(t, result, "t.is_stream_gated = false")
	assert.Contains(t, result, "t.stream_conditions->>'usdc_purchase' IS NOT NULL")
	assert.Contains(t, result, " OR ")

	// Test with invalid condition (should be ignored)
	result = buildGateConditionFilter([]string{"invalid_condition"})
	assert.Equal(t, "", result)

	// Test with mix of valid and invalid conditions
	result = buildGateConditionFilter([]string{"follow", "invalid", "tip"})
	assert.Contains(t, result, "t.stream_conditions->>'follow_user_id' IS NOT NULL")
	assert.Contains(t, result, "t.stream_conditions->>'tip_user_id' IS NOT NULL")
	assert.Contains(t, result, " OR ")
	assert.NotContains(t, result, "invalid")
}
