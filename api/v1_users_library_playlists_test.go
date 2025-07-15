package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUsersLibraryPlaylists(t *testing.T) {
	app := testAppWithFixtures(t)
	var response struct {
		Data []struct {
			Class     string `json:"class"`
			ItemType  string `json:"item_type"`
			ItemID    int32  `json:"item_id"`
			Timestamp string `json:"timestamp"`
			Item      any    `json:"item"`
		} `json:"data"`
	}

	status, body := testGet(t, app, "/v1/full/users/7eP5n/library/playlists", &response)
	assert.Equal(t, 200, status)
	assert.Len(t, response.Data, 1)

	jsonAssert(t, body, map[string]any{
		"data.0.class":              "collection_activity_full_without_tracks",
		"data.0.item_type":          "playlist",
		"data.0.item_id":            1,
		"data.0.item.playlist_name": "First",
	})

	status, body = testGet(t, app, "/v1/full/users/7eP5n/library/playlists?type=favorite", &response)
	assert.Equal(t, 200, status)
	assert.Len(t, response.Data, 1)

	jsonAssert(t, body, map[string]any{
		"data.0.class":              "collection_activity_full_without_tracks",
		"data.0.item_type":          "playlist",
		"data.0.item_id":            1,
		"data.0.item.playlist_name": "First",
	})
}

func TestUsersLibraryAlbums(t *testing.T) {
	app := testAppWithFixtures(t)
	var response struct {
		Data []struct {
			Class     string `json:"class"`
			ItemType  string `json:"item_type"`
			ItemID    int32  `json:"item_id"`
			Timestamp string `json:"timestamp"`
			Item      any    `json:"item"`
		} `json:"data"`
	}

	// Test albums endpoint
	// User 1 owns album 3 ("SecondAlbum") and saves album 2 ("Follow Gated Stream")
	status, body := testGet(t, app, "/v1/full/users/7eP5n/library/albums", &response)
	assert.Equal(t, 200, status)
	assert.Len(t, response.Data, 2)

	jsonAssert(t, body, map[string]any{
		"data.0.class":              "collection_activity_full_without_tracks",
		"data.0.item_type":          "playlist",
		"data.0.item_id":            3,
		"data.0.item.playlist_name": "SecondAlbum",
	})

	jsonAssert(t, body, map[string]any{
		"data.1.class":              "collection_activity_full_without_tracks",
		"data.1.item_type":          "playlist",
		"data.1.item_id":            2,
		"data.1.item.playlist_name": "Follow Gated Stream",
	})
}
