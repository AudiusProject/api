package api

import (
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersSales(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "seller"},
			{"user_id": 2, "handle": "buyer1", "name": "c"},
			{"user_id": 3, "handle": "buyer2", "name": "a"},
			{"user_id": 4, "handle": "buyer3", "name": "b"},
			{"user_id": 5, "handle": "buyer4", "name": "d"},
		},
		"tracks": []map[string]any{
			{"track_id": 1, "title": "b", "owner_id": 1},
			{"track_id": 2, "title": "c", "owner_id": 1},
			{"track_id": 3, "title": "d", "owner_id": 1},
			{"track_id": 4, "title": "a", "owner_id": 1},
		},
		"playlists": []map[string]any{
			{"playlist_id": 1, "playlist_name": "e", "playlist_owner_id": 1},
			{"playlist_id": 2, "playlist_name": "e", "playlist_owner_id": 1, "is_album": true},
		},
		"usdc_purchases": []map[string]any{
			{
				"seller_user_id": 1,
				"buyer_user_id":  5,
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
				"seller_user_id": 1,
				"buyer_user_id":  5,
				"access":         "stream",
				"amount":         2000000,
				"content_type":   "album",
				"content_id":     2,
				"splits":         "[]",
				"created_at":     time.Date(2024, 6, 1, 0, 1, 0, 0, time.UTC),
				"signature":      "faddf",
				"extra_amount":   0,
			},
			{
				"seller_user_id": 1,
				"buyer_user_id":  3,
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
				"seller_user_id": 1,
				"buyer_user_id":  4,
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
				"seller_user_id": 1,
				"buyer_user_id":  2,
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
				"seller_user_id": 1,
				"buyer_user_id":  3,
				"access":         "download",
				"amount":         2000000,
				"content_type":   "track",
				"content_id":     2,
				"splits":         "[{\"user_id\": 1, \"payout_wallet\": \"something\", \"amount\": 1800000, \"percentage\": 100 },{\"payout_wallet\": \"network\", \"amount\": 200000, \"percentage\": 10 }]",
				"created_at":     time.Date(2024, 6, 4, 0, 0, 0, 0, time.UTC),
				"signature":      "def",
				"extra_amount":   0,
			},
		},
	}

	database.Seed(app.pool, fixtures)

	// default sort, check all fields of a couple
	{
		status, body := testGet(t, app, "/v1/users/7eP5n/sales")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.0.seller_user_id": "7eP5n"})
		jsonAssert(t, body, map[string]any{"data.0.buyer_user_id": "lebQD"})
		jsonAssert(t, body, map[string]any{"data.0.content_type": "track"})
		jsonAssert(t, body, map[string]any{"data.0.content_id": "ML51L"})
		jsonAssert(t, body, map[string]any{"data.0.amount": "2000000"})
		jsonAssert(t, body, map[string]any{"data.0.extra_amount": "0"})
		jsonAssert(t, body, map[string]any{"data.0.signature": "def"})
		jsonAssert(t, body, map[string]any{"data.0.access": "download"})
		jsonAssert(t, body, map[string]any{"data.0.splits.0.user_id": 1})
		jsonAssert(t, body, map[string]any{"data.0.splits.0.payout_wallet": "something"})
		jsonAssert(t, body, map[string]any{"data.0.splits.0.amount": "1800000"})
		// hide percentage
		jsonAssert(t, body, map[string]any{"data.0.splits.0.percentage": nil})
		jsonAssert(t, body, map[string]any{"data.0.splits.1.user_id": nil})
		jsonAssert(t, body, map[string]any{"data.0.splits.1.payout_wallet": "network"})
		jsonAssert(t, body, map[string]any{"data.0.splits.1.amount": "200000"})

		jsonAssert(t, body, map[string]any{"data.1.seller_user_id": "7eP5n"})
		jsonAssert(t, body, map[string]any{"data.1.buyer_user_id": "ML51L"})
		jsonAssert(t, body, map[string]any{"data.1.content_type": "track"})
		jsonAssert(t, body, map[string]any{"data.1.content_id": "7eP5n"})
		jsonAssert(t, body, map[string]any{"data.1.amount": "1000000"})
		jsonAssert(t, body, map[string]any{"data.1.extra_amount": "1000000"})
		jsonAssert(t, body, map[string]any{"data.1.signature": "abc"})
		jsonAssert(t, body, map[string]any{"data.1.access": "stream"})

		jsonAssert(t, body, map[string]any{"data.2.content_id": "ELKzn"})
		jsonAssert(t, body, map[string]any{"data.3.content_id": "lebQD"})
	}

	// reverse sort (asc)
	{
		status, body := testGet(t, app, "/v1/users/7eP5n/sales?sort_direction=asc")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.0.content_id": "7eP5n", "data.0.content_type": "playlist"})
		jsonAssert(t, body, map[string]any{"data.1.content_id": "ML51L", "data.1.content_type": "album"})
		jsonAssert(t, body, map[string]any{"data.2.content_id": "lebQD"})
		jsonAssert(t, body, map[string]any{"data.3.content_id": "ELKzn"})
		jsonAssert(t, body, map[string]any{"data.4.content_id": "7eP5n"})
		jsonAssert(t, body, map[string]any{"data.5.content_id": "ML51L"})
	}

	// content title sort (asc)
	{
		status, body := testGet(t, app, "/v1/users/7eP5n/sales?sort_method=content_title&sort_direction=asc")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.0.content_id": "ELKzn"})
		jsonAssert(t, body, map[string]any{"data.1.content_id": "7eP5n"})
		jsonAssert(t, body, map[string]any{"data.2.content_id": "ML51L"})
		jsonAssert(t, body, map[string]any{"data.3.content_id": "lebQD"})
		jsonAssert(t, body, map[string]any{"data.4.content_id": "7eP5n"})
		jsonAssert(t, body, map[string]any{"data.5.content_id": "ML51L"})
	}

	// content title sort (desc)
	{
		status, body := testGet(t, app, "/v1/users/7eP5n/sales?sort_method=content_title&sort_direction=desc")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.0.content_id": "ML51L"})
		jsonAssert(t, body, map[string]any{"data.1.content_id": "7eP5n"})
		jsonAssert(t, body, map[string]any{"data.2.content_id": "lebQD"})
		jsonAssert(t, body, map[string]any{"data.3.content_id": "ML51L"})
		jsonAssert(t, body, map[string]any{"data.4.content_id": "7eP5n"})
		jsonAssert(t, body, map[string]any{"data.5.content_id": "ELKzn"})
	}

	// content filters
	{
		status, body := testGet(t, app, "/v1/users/7eP5n/sales?content_ids=lebQD&content_ids=ML51L&content_type=track")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data.0.content_id": "ML51L"})
		jsonAssert(t, body, map[string]any{"data.1.content_id": "lebQD"})
		jsonAssert(t, body, map[string]any{"data.2.content_id": nil})
	}
}
