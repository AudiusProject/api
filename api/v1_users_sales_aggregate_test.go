package api

import (
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersSalesAggregate(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id": 100,
				"owner_id": 1,
				"title":    "Popular Track",
			},
			{
				"track_id": 101,
				"owner_id": 1,
				"title":    "Less Popular Track",
			},
		},
		"playlists": []map[string]any{
			{
				"playlist_id":       200,
				"playlist_name":     "Popular Playlist",
				"playlist_owner_id": 1,
			},
		},
		"users": []map[string]any{
			{
				"user_id":   1,
				"handle":    "seller",
				"handle_lc": "seller",
			},
			{
				"user_id":   2,
				"handle":    "buyer1",
				"handle_lc": "buyer1",
			},
			{
				"user_id":   3,
				"handle":    "buyer2",
				"handle_lc": "buyer2",
			},
			{
				"user_id":   4,
				"handle":    "buyer3",
				"handle_lc": "buyer3",
			},
		},
		"usdc_purchases": []map[string]any{
			// Track 100: 3 purchases (most popular)
			{
				"slot":           1,
				"signature":      "purchase1",
				"seller_user_id": 1,
				"buyer_user_id":  2,
				"content_type":   "track",
				"content_id":     100,
				"amount":         2000000,
				"splits":         "[]",
			},
			{
				"slot":           2,
				"signature":      "purchase2",
				"seller_user_id": 1,
				"buyer_user_id":  3,
				"content_type":   "track",
				"content_id":     100,
				"amount":         2000000,
				"splits":         "[]",
			},
			{
				"slot":           3,
				"signature":      "purchase3",
				"seller_user_id": 1,
				"buyer_user_id":  4,
				"content_type":   "track",
				"content_id":     100,
				"amount":         2000000,
				"splits":         "[]",
			},
			// Playlist 200: 2 purchases (second most popular)
			{
				"slot":           4,
				"signature":      "purchase4",
				"seller_user_id": 1,
				"buyer_user_id":  2,
				"content_type":   "playlist",
				"content_id":     200,
				"amount":         5000000,
				"splits":         "[]",
			},
			{
				"slot":           5,
				"signature":      "purchase5",
				"seller_user_id": 1,
				"buyer_user_id":  3,
				"content_type":   "playlist",
				"content_id":     200,
				"amount":         5000000,
				"splits":         "[]",
			},
			// Track 101: 1 purchase (least popular)
			{
				"slot":           6,
				"signature":      "purchase6",
				"seller_user_id": 1,
				"buyer_user_id":  4,
				"content_type":   "track",
				"content_id":     101,
				"amount":         1000000,
				"splits":         "[]",
			},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	var response struct {
		Data []SalesAggregateData
	}

	// Test getting all sales aggregate for user 1 (seller)
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/sales/aggregate", &response)
		assert.Equal(t, 200, status)
		assert.Len(t, response.Data, 3)

		// Results should be ordered by purchase_count DESC
		// Track 100: 3 purchases
		assert.Equal(t, trashid.HashId(100), response.Data[0].ContentID)
		assert.Equal(t, "track", response.Data[0].ContentType)
		assert.Equal(t, 3, response.Data[0].PurchaseCount)

		// Playlist 200: 2 purchases
		assert.Equal(t, trashid.HashId(200), response.Data[1].ContentID)
		assert.Equal(t, "playlist", response.Data[1].ContentType)
		assert.Equal(t, 2, response.Data[1].PurchaseCount)

		// Track 101: 1 purchase
		assert.Equal(t, trashid.HashId(101), response.Data[2].ContentID)
		assert.Equal(t, "track", response.Data[2].ContentType)
		assert.Equal(t, 1, response.Data[2].PurchaseCount)
	}

	// Test limit parameter
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/sales/aggregate?limit=2", &response)
		assert.Equal(t, 200, status)
		assert.Len(t, response.Data, 2)

		// Should get the top 2 by purchase count
		assert.Equal(t, trashid.HashId(100), response.Data[0].ContentID)
		assert.Equal(t, 3, response.Data[0].PurchaseCount)
		assert.Equal(t, trashid.HashId(200), response.Data[1].ContentID)
		assert.Equal(t, 2, response.Data[1].PurchaseCount)
	}

	// Test offset parameter
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/sales/aggregate?offset=1&limit=2", &response)
		assert.Equal(t, 200, status)
		assert.Len(t, response.Data, 2)

		// Should skip the first result and get the next 2
		assert.Equal(t, trashid.HashId(200), response.Data[0].ContentID)
		assert.Equal(t, 2, response.Data[0].PurchaseCount)
		assert.Equal(t, trashid.HashId(101), response.Data[1].ContentID)
		assert.Equal(t, 1, response.Data[1].PurchaseCount)
	}

	// Test user with no sales
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(2)+"/sales/aggregate", &response)
		assert.Equal(t, 200, status)
		assert.Len(t, response.Data, 0)
	}
}
