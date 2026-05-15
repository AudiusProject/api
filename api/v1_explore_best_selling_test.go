package api

import (
	"fmt"
	"testing"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestExploreBestSelling(t *testing.T) {
	app := emptyTestApp(t)

	users := make([]map[string]any, 10)
	for i := range 10 {
		userId := i + 1
		users[i] = map[string]any{
			"user_id":   userId,
			"handle":    fmt.Sprintf("user%d", userId),
			"handle_lc": fmt.Sprintf("user%d", userId),
			"wallet":    fmt.Sprintf("0x%d", userId),
		}
	}

	tracks := make([]map[string]any, 10)
	for i := range 10 {
		trackId := i + 1
		ownerId := i + 1
		tracks[i] = map[string]any{
			"track_id": trackId,
			"owner_id": ownerId,
			"title":    fmt.Sprintf("Track %d", trackId),
		}
	}
	// Track 3 and 4 are deleted and unlisted
	tracks[2]["is_delete"] = true
	tracks[3]["is_unlisted"] = true

	albums := make([]map[string]any, 10)
	for i := range 10 {
		albumId := i + 1
		ownerId := i + 1
		albums[i] = map[string]any{
			"playlist_id":       albumId,
			"playlist_owner_id": ownerId,
			"playlist_name":     fmt.Sprintf("Album %d", albumId),
			"is_album":          true,
		}
	}
	// Album 3 and 4 are deleted and unlisted
	albums[2]["is_delete"] = true
	albums[3]["is_private"] = true

	fixtures := database.FixtureMap{
		"users":     users,
		"tracks":    tracks,
		"playlists": albums,
		"sol_purchases": {
			// Track 1: 5 purchases
			{"signature": "0x1", "instruction_index": 0, "buyer_user_id": 6, "content_id": 1, "content_type": "track", "amount": 1000, "is_valid": true},
			{"signature": "0x2", "instruction_index": 0, "buyer_user_id": 7, "content_id": 1, "content_type": "track", "amount": 1000, "is_valid": true},
			{"signature": "0x3", "instruction_index": 0, "buyer_user_id": 8, "content_id": 1, "content_type": "track", "amount": 1000, "is_valid": true},
			{"signature": "0x4", "instruction_index": 0, "buyer_user_id": 9, "content_id": 1, "content_type": "track", "amount": 1000, "is_valid": true},
			{"signature": "0x5", "instruction_index": 0, "buyer_user_id": 10, "content_id": 1, "content_type": "track", "amount": 1000, "is_valid": true},
			// Album 1: 4 purchases
			{"signature": "0x6", "instruction_index": 0, "buyer_user_id": 6, "content_id": 1, "content_type": "album", "amount": 2000, "is_valid": true},
			{"signature": "0x7", "instruction_index": 0, "buyer_user_id": 7, "content_id": 1, "content_type": "album", "amount": 2000, "is_valid": true},
			{"signature": "0x8", "instruction_index": 0, "buyer_user_id": 8, "content_id": 1, "content_type": "album", "amount": 2000, "is_valid": true},
			{"signature": "0x9", "instruction_index": 0, "buyer_user_id": 9, "content_id": 1, "content_type": "album", "amount": 2000, "is_valid": true},
			// Track 2: 3 purchases
			{"signature": "0x10", "instruction_index": 0, "buyer_user_id": 6, "content_id": 2, "content_type": "track", "amount": 1000, "is_valid": true},
			{"signature": "0x11", "instruction_index": 0, "buyer_user_id": 7, "content_id": 2, "content_type": "track", "amount": 1000, "is_valid": true},
			{"signature": "0x12", "instruction_index": 0, "buyer_user_id": 8, "content_id": 2, "content_type": "track", "amount": 1000, "is_valid": true},
			// Album 2: 2 purchases
			{"signature": "0x13", "instruction_index": 0, "buyer_user_id": 6, "content_id": 2, "content_type": "album", "amount": 2000, "is_valid": true},
			{"signature": "0x14", "instruction_index": 0, "buyer_user_id": 7, "content_id": 2, "content_type": "album", "amount": 2000, "is_valid": true},
			// Track 5: 1 purchase
			{"signature": "0x15", "instruction_index": 0, "buyer_user_id": 6, "content_id": 5, "content_type": "track", "amount": 1000, "is_valid": true},
			// Album 5: 1 purchase
			{"signature": "0x16", "instruction_index": 0, "buyer_user_id": 6, "content_id": 5, "content_type": "album", "amount": 1000, "is_valid": true},
			// Track 3 (deleted): 1 purchase
			{"signature": "0x17", "instruction_index": 0, "buyer_user_id": 6, "content_id": 3, "content_type": "track", "amount": 1000, "is_valid": true},
			// Track 4 (unlisted): 1 purchase
			{"signature": "0x18", "instruction_index": 0, "buyer_user_id": 7, "content_id": 4, "content_type": "track", "amount": 1000, "is_valid": true},
			// Album 3 (deleted): 1 purchase
			{"signature": "0x19", "instruction_index": 0, "buyer_user_id": 6, "content_id": 3, "content_type": "album", "amount": 2000, "is_valid": true},
			// Album 4 (unlisted): 1 purchase
			{"signature": "0x20", "instruction_index": 0, "buyer_user_id": 7, "content_id": 4, "content_type": "album", "amount": 2000, "is_valid": true},
			// Track 6 (purchases from before 6mo cutoff): 5 purchases
			{"signature": "0x21", "instruction_index": 0, "buyer_user_id": 6, "content_id": 6, "content_type": "track", "amount": 1000, "created_at": time.Now().AddDate(0, -7, 0), "is_valid": true},
			{"signature": "0x22", "instruction_index": 0, "buyer_user_id": 7, "content_id": 6, "content_type": "track", "amount": 1000, "created_at": time.Now().AddDate(0, -7, 0), "is_valid": true},
			{"signature": "0x23", "instruction_index": 0, "buyer_user_id": 8, "content_id": 6, "content_type": "track", "amount": 1000, "created_at": time.Now().AddDate(0, -7, 0), "is_valid": true},
			{"signature": "0x24", "instruction_index": 0, "buyer_user_id": 9, "content_id": 6, "content_type": "track", "amount": 1000, "created_at": time.Now().AddDate(0, -7, 0), "is_valid": true},
			{"signature": "0x25", "instruction_index": 0, "buyer_user_id": 10, "content_id": 6, "content_type": "track", "amount": 1000, "created_at": time.Now().AddDate(0, -7, 0), "is_valid": true},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	{
		var BestSellingResponse struct {
			Data    []BestSellingItem
			Related struct {
				Tracks    []dbv1.Track
				Playlists []dbv1.Playlist
			}
		}

		status, body := testGet(t, app, "/v1/full/explore/best-selling", &BestSellingResponse)
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":              6,
			"data.0.content_id":   trashid.MustEncodeHashID(1),
			"data.0.content_type": "track",
			"data.1.content_id":   trashid.MustEncodeHashID(1),
			"data.1.content_type": "album",
			"data.2.content_id":   trashid.MustEncodeHashID(2),
			"data.2.content_type": "track",
			"data.3.content_id":   trashid.MustEncodeHashID(2),
			"data.3.content_type": "album",
			"data.4.content_id":   trashid.MustEncodeHashID(5),
			"data.4.content_type": "track",
			"data.5.content_id":   trashid.MustEncodeHashID(5),
			"data.5.content_type": "album",
		})

		jsonAssert(t, body, map[string]any{
			"related.tracks.#":    3,
			"related.playlists.#": 3,
			// Note: not checking IDs because they are not in a deterministic order
		})
	}

	// Remaining tests use min endpoints
	{
		status, body := testGet(t, app, "/v1/explore/best-selling?type=all&limit=2")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":              2,
			"data.0.content_id":   trashid.MustEncodeHashID(1),
			"data.0.content_type": "track",
			"data.1.content_id":   trashid.MustEncodeHashID(1),
			"data.1.content_type": "album",
		})
	}

	{
		status, body := testGet(t, app, "/v1/explore/best-selling?limit=1&offset=3")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":              1,
			"data.0.content_id":   trashid.MustEncodeHashID(2),
			"data.0.content_type": "album",
		})
	}

	{
		status, body := testGet(t, app, "/v1/explore/best-selling?type=track")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":              3,
			"data.0.content_id":   trashid.MustEncodeHashID(1),
			"data.0.content_type": "track",
			"data.1.content_id":   trashid.MustEncodeHashID(2),
			"data.1.content_type": "track",
			"data.2.content_id":   trashid.MustEncodeHashID(5),
			"data.2.content_type": "track",
		})
	}

	{
		status, body := testGet(t, app, "/v1/explore/best-selling?type=album")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":              3,
			"data.0.content_id":   trashid.MustEncodeHashID(1),
			"data.0.content_type": "album",
			"data.1.content_id":   trashid.MustEncodeHashID(2),
			"data.1.content_type": "album",
			"data.2.content_id":   trashid.MustEncodeHashID(5),
			"data.2.content_type": "album",
		})
	}

}

func TestExploreBestSellingInvalidParams(t *testing.T) {
	app := emptyTestApp(t)

	{
		status, _ := testGet(t, app, "/v1/explore/best-selling?type=invalid")
		assert.Equal(t, 400, status)
	}

	{
		status, _ := testGet(t, app, "/v1/explore/best-selling?limit=-1")
		assert.Equal(t, 400, status)
	}

	{
		status, _ := testGet(t, app, "/v1/explore/best-selling?limit=101")
		assert.Equal(t, 400, status)
	}

	{
		status, _ := testGet(t, app, "/v1/explore/best-selling?offset=-1")
		assert.Equal(t, 400, status)
	}

	{
		status, _ := testGet(t, app, "/v1/explore/best-selling?type=invalid")
		assert.Equal(t, 400, status)
	}
}
