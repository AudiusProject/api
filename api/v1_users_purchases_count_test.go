package api

import (
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersPurchasesCount(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "buyer"},
			{"user_id": 2, "handle": "seller1", "name": "c"},
			{"user_id": 3, "handle": "seller2", "name": "a"},
			{"user_id": 4, "handle": "seller3", "name": "b"},
			{"user_id": 5, "handle": "seller4", "name": "d"},
		},
		"tracks": []map[string]any{
			{"track_id": 1, "title": "b", "owner_id": 2},
			{"track_id": 2, "title": "c", "owner_id": 3},
			{"track_id": 3, "title": "d", "owner_id": 3},
			{"track_id": 4, "title": "a", "owner_id": 4},
		},
		"playlists": []map[string]any{
			{"playlist_id": 1, "playlist_name": "e", "playlist_owner_id": 5},
			{"playlist_id": 2, "playlist_name": "f", "playlist_owner_id": 5, "is_album": true},
		},
		"usdc_purchases": []map[string]any{
			{
				"seller_user_id": 5,
				"buyer_user_id":  1,
				"access":         "stream",
				"amount":         2000000,
				"content_type":   "playlist",
				"content_id":     1,
				"splits":         "[]",
				"created_at":     time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
				"signature":      "gfsgf",
				"extra_amount":   0,
			},
			{
				"seller_user_id": 5,
				"buyer_user_id":  1,
				"access":         "stream",
				"amount":         2000000,
				"content_type":   "album",
				"content_id":     2,
				"splits":         "[]",
				"created_at":     time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC),
				"signature":      "faddf",
				"extra_amount":   0,
			},
			{
				"seller_user_id": 3,
				"buyer_user_id":  1,
				"access":         "stream",
				"amount":         2000000,
				"content_type":   "track",
				"content_id":     3,
				"splits":         "[]",
				"created_at":     time.Date(2024, 6, 1, 1, 0, 0, 0, time.UTC),
				"signature":      "adfdgad",
				"extra_amount":   0,
			},
			{
				"seller_user_id": 4,
				"buyer_user_id":  1,
				"access":         "stream",
				"amount":         2000000,
				"content_type":   "track",
				"content_id":     4,
				"splits":         "[]",
				"created_at":     time.Date(2024, 6, 2, 0, 0, 0, 0, time.UTC),
				"signature":      "agadgafh",
				"extra_amount":   0,
			},
			{
				"seller_user_id": 2,
				"buyer_user_id":  1,
				"access":         "stream",
				"amount":         1000000,
				"content_type":   "track",
				"content_id":     1,
				"splits":         "[]",
				"created_at":     time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC),
				"signature":      "abc",
				"extra_amount":   1000000,
			},
			{
				"seller_user_id": 3,
				"buyer_user_id":  1,
				"access":         "download",
				"amount":         2000000,
				"content_type":   "track",
				"content_id":     2,
				"splits":         "[{\"user_id\": 3, \"payout_wallet\": \"something\", \"amount\": 1800000, \"percentage\": 100 },{\"payout_wallet\": \"network\", \"amount\": 200000, \"percentage\": 10 }]",
				"created_at":     time.Date(2024, 6, 4, 0, 0, 0, 0, time.UTC),
				"signature":      "def",
				"extra_amount":   0,
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	{
		status, body := testGet(t, app, "/v1/users/7eP5n/purchases/count")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data": 6})
	}

	// with content id filters
	{
		status, body := testGet(t, app, "/v1/users/7eP5n/purchases/count?content_ids=7eP5n&content_ids=ML51L")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data": 4})
	}

	// with content type filter (playlist)
	{
		status, body := testGet(t, app, "/v1/users/7eP5n/purchases/count?content_type=playlist")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data": 1})
	}

	// with content type filter (track)
	{
		status, body := testGet(t, app, "/v1/users/7eP5n/purchases/count?content_type=track")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data": 4})
	}

	// with content type filter (album)
	{
		status, body := testGet(t, app, "/v1/users/7eP5n/purchases/count?content_type=album")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data": 1})
	}
}
