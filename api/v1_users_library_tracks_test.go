package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestUsersLibraryTracks(t *testing.T) {
	app := testAppWithFixtures(t)
	var response struct {
		Data []struct {
			Class     string `json:"class"`
			ItemType  string `json:"item_type"`
			ItemID    int32  `json:"item_id"`
			Timestamp string `json:"timestamp"`
			Item      struct {
				ID    string `json:"id"`
				Title string `json:"title"`
			} `json:"item"`
		} `json:"data"`
	}

	// User 1 has saved track 100 (T1) and reposted track 200 (Culca Canyon)
	user1Id := trashid.MustEncodeHashID(1)

	// Test all library tracks
	status, body := testGet(t, app, "/v1/full/users/"+user1Id+"/library/tracks?type=all", &response)
	assert.Equal(t, 200, status)
	assert.GreaterOrEqual(t, len(response.Data), 2, "Should have at least saved and reposted tracks")

	jsonAssert(t, body, map[string]any{
		"data.0.class":     "track_activity_full",
		"data.0.item_type": "track",
	})

	// Test favorite tracks only
	status, body = testGet(t, app, "/v1/full/users/"+user1Id+"/library/tracks?type=favorite", &response)
	assert.Equal(t, 200, status)
	assert.GreaterOrEqual(t, len(response.Data), 1, "Should have at least one favorite track")

	jsonAssert(t, body, map[string]any{
		"data.0.class":     "track_activity_full",
		"data.0.item_type": "track",
		"data.0.item_id":   100, // Track 100 (T1) is saved
	})

	// Test repost tracks only
	status, body = testGet(t, app, "/v1/full/users/"+user1Id+"/library/tracks?type=repost", &response)
	assert.Equal(t, 200, status)
	assert.GreaterOrEqual(t, len(response.Data), 1, "Should have at least one reposted track")

	jsonAssert(t, body, map[string]any{
		"data.0.class":     "track_activity_full",
		"data.0.item_type": "track",
		"data.0.item_id":   200, // Track 200 (Culca Canyon) is reposted
	})

	// Test purchase tracks only (user 11 has purchased track 303)
	user11Id := trashid.MustEncodeHashID(11)
	status, body = testGet(t, app, "/v1/full/users/"+user11Id+"/library/tracks?type=purchase", &response)
	assert.Equal(t, 200, status)
	assert.GreaterOrEqual(t, len(response.Data), 1, "Should have at least one purchased track")

	jsonAssert(t, body, map[string]any{
		"data.0.class":     "track_activity_full",
		"data.0.item_type": "track",
		"data.0.item_id":   303, // Track 303 (Pay Gated Stream) is purchased
	})
}

func TestUsersLibraryTracksUnlistedFiltered(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id": 1,
				"handle":  "user1",
			},
		},
		"tracks": []map[string]any{
			{
				"track_id":    100,
				"title":       "Public Track",
				"owner_id":    1,
				"is_unlisted": false,
			},
			{
				"track_id":    201,
				"title":       "Unlisted Track",
				"owner_id":    1,
				"is_unlisted": true,
			},
		},
		"saves": []map[string]any{
			{
				"user_id":      1,
				"save_item_id": 100,
				"save_type":    "track",
			},
			{
				"user_id":      1,
				"save_item_id": 201, // Save an unlisted track
				"save_type":    "track",
			},
		},
		"usdc_purchases": []map[string]any{
			{
				"signature":      "test1",
				"buyer_user_id":  1,
				"seller_user_id": 1,
				"content_id":     201, // Purchase an unlisted track
				"content_type":   "track",
				"amount":         100,
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)
	user1Id := trashid.MustEncodeHashID(1)

	var response struct {
		Data []struct {
			ItemID int32 `json:"item_id"`
		} `json:"data"`
	}

	// Test that unlisted tracks are filtered out from favorites
	status, _ := testGet(t, app, "/v1/full/users/"+user1Id+"/library/tracks?type=favorite", &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(response.Data), "Should only have public track")
	assert.Equal(t, int32(100), response.Data[0].ItemID, "Should only return public track")

	// Test that unlisted purchased tracks are filtered out
	status, _ = testGet(t, app, "/v1/full/users/"+user1Id+"/library/tracks?type=purchase", &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 0, len(response.Data), "Should not return unlisted purchased tracks")

	// Test that unlisted tracks are filtered out from all
	status, _ = testGet(t, app, "/v1/full/users/"+user1Id+"/library/tracks?type=all", &response)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(response.Data), "Should only have public track")
	assert.Equal(t, int32(100), response.Data[0].ItemID, "Should only return public track")
}

func TestUsersLibraryTracksSorting(t *testing.T) {
	app := testAppWithFixtures(t)
	user1Id := trashid.MustEncodeHashID(1)

	var response struct {
		Data []struct {
			ItemID int32 `json:"item_id"`
			Item   struct {
				Title string `json:"title"`
			} `json:"item"`
		} `json:"data"`
	}

	// Test sorting by title
	status, _ := testGet(t, app, "/v1/full/users/"+user1Id+"/library/tracks?type=all&sort_method=title&sort_direction=asc", &response)
	assert.Equal(t, 200, status)
	assert.GreaterOrEqual(t, len(response.Data), 2, "Should have multiple tracks")

	// Verify tracks are sorted by title ascending
	if len(response.Data) >= 2 {
		assert.LessOrEqual(t, response.Data[0].Item.Title, response.Data[1].Item.Title,
			"Tracks should be sorted by title ascending")
	}

	// Test sorting by added_date (default)
	status, _ = testGet(t, app, "/v1/full/users/"+user1Id+"/library/tracks?type=all&sort_method=added_date&sort_direction=desc", &response)
	assert.Equal(t, 200, status)
	assert.GreaterOrEqual(t, len(response.Data), 2, "Should have multiple tracks")
}

func TestUsersLibraryTracksQuery(t *testing.T) {
	app := testAppWithFixtures(t)
	user1Id := trashid.MustEncodeHashID(1)

	var response struct {
		Data []struct {
			ItemID int32 `json:"item_id"`
			Item   struct {
				Title string `json:"title"`
			} `json:"item"`
		} `json:"data"`
	}

	// Test query filtering by track title
	status, _ := testGet(t, app, "/v1/full/users/"+user1Id+"/library/tracks?type=all&query=T1", &response)
	assert.Equal(t, 200, status)
	assert.GreaterOrEqual(t, len(response.Data), 1, "Should find tracks matching query")

	// Verify all results match the query
	for _, item := range response.Data {
		assert.Contains(t, item.Item.Title, "T1", "All results should match query")
	}
}

func TestUsersLibraryTracksMetadataNotNull(t *testing.T) {
	app := testAppWithFixtures(t)
	user1Id := trashid.MustEncodeHashID(1)

	var response struct {
		Data []struct {
			ItemID int32 `json:"item_id"`
			Item   any   `json:"item"`
		} `json:"data"`
	}

	// Test that all returned tracks have non-null metadata
	status, _ := testGet(t, app, "/v1/full/users/"+user1Id+"/library/tracks?type=all", &response)
	assert.Equal(t, 200, status)
	assert.GreaterOrEqual(t, len(response.Data), 1, "Should have at least one track")

	// Verify all items have non-null metadata
	for _, item := range response.Data {
		assert.NotNil(t, item.Item, "Track metadata should not be null for item_id %d", item.ItemID)
	}
}
