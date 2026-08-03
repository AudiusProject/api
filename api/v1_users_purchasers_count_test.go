package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersPurchasersCount(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id": 100,
				"owner_id": 1,
				"title":    "Seller Track 1",
			},
			{
				"track_id": 101,
				"owner_id": 1,
				"title":    "Seller Track 2",
			},
		},
		"playlists": []map[string]any{
			{
				"playlist_id":       200,
				"playlist_name":     "Seller Playlist 1",
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
				"wallet":    "0x1234567890abcdef",
			},
			{
				"user_id":   3,
				"handle":    "buyer2",
				"handle_lc": "buyer2",
				"wallet":    "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
			},
			{
				"user_id":   4,
				"handle":    "buyer3",
				"handle_lc": "buyer3",
				"wallet":    "0x9876543210fedcba",
			},
		},
		"sol_purchases": []map[string]any{
			{"slot": 1, "signature": "purchase1", "instruction_index": 0, "buyer_user_id": 2, "content_type": "track", "content_id": 100, "amount": 2000000, "is_valid": true},
			{"slot": 2, "signature": "purchase2", "instruction_index": 0, "buyer_user_id": 3, "content_type": "track", "content_id": 101, "amount": 2000000, "is_valid": true},
			{"slot": 3, "signature": "purchase3", "instruction_index": 0, "buyer_user_id": 4, "content_type": "track", "content_id": 100, "amount": 2000000, "is_valid": true},
			{"slot": 4, "signature": "purchase4", "instruction_index": 0, "buyer_user_id": 2, "content_type": "playlist", "content_id": 200, "amount": 5000000, "is_valid": true},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	// Test count all purchasers for user 1 (seller)
	{
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers/count")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 3,
		})
	}

	// Test count purchasers filtered by content_type=track
	{
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers/count?content_type=track")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 3,
		})
	}

	// Test count purchasers filtered by content_type=playlist
	{
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers/count?content_type=playlist")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 1,
		})
	}

	// Test count purchasers filtered by content_id=100
	{
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers/count?content_id=100")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 2,
		})
	}

	// Test count purchasers filtered by content_id=101
	{
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers/count?content_id=101")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 1,
		})
	}

	// Test combined filters: content_type=track and content_id=100
	{
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers/count?content_type=track&content_id=100")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 2,
		})
	}

	// Test non-existent content filter
	{
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers/count?content_id=999")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 0,
		})
	}
}
