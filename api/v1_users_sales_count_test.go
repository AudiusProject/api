package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersSalesCount(t *testing.T) {
	user1Wallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "seller", "wallet": user1Wallet},
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
		"sol_purchases": []map[string]any{
			{"signature": "gfsgf", "instruction_index": 0, "buyer_user_id": 5, "amount": 2000000, "content_type": "playlist", "content_id": 1, "created_at": time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), "is_valid": true},
			{"signature": "faddf", "instruction_index": 0, "buyer_user_id": 5, "amount": 2000000, "content_type": "album", "content_id": 2, "created_at": time.Date(2024, 6, 1, 0, 1, 0, 0, time.UTC), "is_valid": true},
			{"signature": "adfdgad", "instruction_index": 0, "buyer_user_id": 3, "amount": 2000000, "content_type": "track", "content_id": 3, "created_at": time.Date(2024, 6, 1, 1, 0, 0, 0, time.UTC), "is_valid": true},
			{"signature": "agadgafh", "instruction_index": 0, "buyer_user_id": 4, "amount": 2000000, "content_type": "track", "content_id": 4, "created_at": time.Date(2024, 6, 2, 0, 0, 0, 0, time.UTC), "is_valid": true},
			{"signature": "abc", "instruction_index": 0, "buyer_user_id": 2, "amount": 1000000, "content_type": "track", "content_id": 1, "created_at": time.Date(2024, 6, 3, 0, 0, 0, 0, time.UTC), "is_valid": true},
			{"signature": "def", "instruction_index": 0, "buyer_user_id": 3, "amount": 2000000, "content_type": "track", "content_id": 2, "access_type": "download", "created_at": time.Date(2024, 6, 4, 0, 0, 0, 0, time.UTC), "is_valid": true},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	{
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/sales/count", user1Wallet)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data": 6})
	}

	// with content id filters
	{
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/sales/count?content_ids=7eP5n&content_ids=ML51L", user1Wallet)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data": 4})
	}

	// with content type filter (playlist)
	{
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/sales/count?content_type=playlist", user1Wallet)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data": 1})
	}

	// with content type filter (track)
	{
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/sales/count?content_type=track", user1Wallet)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data": 4})
	}

	// with content type filter (album)
	{
		status, body := testGetWithWallet(t, app, "/v1/users/7eP5n/sales/count?content_type=album", user1Wallet)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{"data": 1})
	}

	// should 403 with bad wallet
	{
		status, _ := testGet(t, app, "/v1/users/7eP5n/sales/count")
		assert.Equal(t, 403, status)
	}
}
