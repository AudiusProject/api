package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersPurchasers(t *testing.T) {
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
		"usdc_purchases": []map[string]any{
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
				"content_id":     101,
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
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	var userResponse struct {
		Data []dbv1.FullUser
	}

	// Test getting all purchasers for user 1 (seller)
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 3)
		// Results should be ordered by buyer_user_id ASC
		assert.Equal(t, "buyer1", userResponse.Data[0].Handle.String)
		assert.Equal(t, "buyer2", userResponse.Data[1].Handle.String)
		assert.Equal(t, "buyer3", userResponse.Data[2].Handle.String)
	}

	// Test getting purchasers filtered by content_type=track
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers?content_type=track", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 3)
		// All buyers have purchased tracks
		assert.Equal(t, "buyer1", userResponse.Data[0].Handle.String)
		assert.Equal(t, "buyer2", userResponse.Data[1].Handle.String)
		assert.Equal(t, "buyer3", userResponse.Data[2].Handle.String)
	}

	// Test getting purchasers filtered by content_type=playlist
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers?content_type=playlist", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 1)
		// Only buyer1 (user 2) purchased a playlist
		assert.Equal(t, "buyer1", userResponse.Data[0].Handle.String)
	}

	// Test getting purchasers filtered by content_id=100
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers?content_id=100", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 2)
		// buyer1 (user 2) and buyer3 (user 4) purchased track 100
		assert.Equal(t, "buyer1", userResponse.Data[0].Handle.String)
		assert.Equal(t, "buyer3", userResponse.Data[1].Handle.String)
	}

	// Test getting purchasers filtered by content_id=101
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers?content_id=101", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 1)
		// Only buyer2 (user 3) purchased track 101
		assert.Equal(t, "buyer2", userResponse.Data[0].Handle.String)
	}

	// Test combined filters: content_type=track and content_id=100
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers?content_type=track&content_id=100", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 2)
		// buyer1 (user 2) and buyer3 (user 4) purchased track 100
		assert.Equal(t, "buyer1", userResponse.Data[0].Handle.String)
		assert.Equal(t, "buyer3", userResponse.Data[1].Handle.String)
	}

	// Test non-existent content filter
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers?content_id=999", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 0)
	}

	// Test limit parameter
	{
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/purchasers?limit=2", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 2)
		assert.Equal(t, "buyer1", userResponse.Data[0].Handle.String)
		assert.Equal(t, "buyer2", userResponse.Data[1].Handle.String)
	}
}
