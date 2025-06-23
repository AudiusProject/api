package api

import (
	"fmt"
	"testing"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
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

	fixtures := FixtureMap{
		"users":     users,
		"tracks":    tracks,
		"playlists": albums,
		"usdc_purchases": {
			// Track 1: 5 purchases
			{
				"buyer_user_id":  6,
				"seller_user_id": 1,
				"content_id":     1,
				"content_type":   "track",
				"amount":         1000,
				"signature":      "0x1",
			},
			{
				"buyer_user_id":  7,
				"seller_user_id": 1,
				"content_id":     1,
				"content_type":   "track",
				"amount":         1000,
				"signature":      "0x2",
			},
			{
				"buyer_user_id":  8,
				"seller_user_id": 1,
				"content_id":     1,
				"content_type":   "track",
				"amount":         1000,
				"signature":      "0x3",
			},
			{
				"buyer_user_id":  9,
				"seller_user_id": 1,
				"content_id":     1,
				"content_type":   "track",
				"amount":         1000,
				"signature":      "0x4",
			},
			{
				"buyer_user_id":  10,
				"seller_user_id": 1,
				"content_id":     1,
				"content_type":   "track",
				"amount":         1000,
				"signature":      "0x5",
			},
			// Album 1: 4 purchases
			{
				"buyer_user_id":  6,
				"seller_user_id": 1,
				"content_id":     1,
				"content_type":   "album",
				"amount":         2000,
				"signature":      "0x6",
			},
			{
				"buyer_user_id":  7,
				"seller_user_id": 1,
				"content_id":     1,
				"content_type":   "album",
				"amount":         2000,
				"signature":      "0x7",
			},
			{
				"buyer_user_id":  8,
				"seller_user_id": 1,
				"content_id":     1,
				"content_type":   "album",
				"amount":         2000,
				"signature":      "0x8",
			},
			{
				"buyer_user_id":  9,
				"seller_user_id": 1,
				"content_id":     1,
				"content_type":   "album",
				"amount":         2000,
				"signature":      "0x9",
			},
			// Track 2: 3 purchases
			{
				"buyer_user_id":  6,
				"seller_user_id": 2,
				"content_id":     2,
				"content_type":   "track",
				"amount":         1000,
				"signature":      "0x10",
			},
			{
				"buyer_user_id":  7,
				"seller_user_id": 2,
				"content_id":     2,
				"content_type":   "track",
				"amount":         1000,
				"signature":      "0x11",
			},
			{
				"buyer_user_id":  8,
				"seller_user_id": 2,
				"content_id":     2,
				"content_type":   "track",
				"amount":         1000,
				"signature":      "0x12",
			},
			// Album 2: 2 purchases
			{
				"buyer_user_id":  6,
				"seller_user_id": 2,
				"content_id":     2,
				"content_type":   "album",
				"amount":         2000,
				"signature":      "0x13",
			},
			{
				"buyer_user_id":  7,
				"seller_user_id": 2,
				"content_id":     2,
				"content_type":   "album",
				"amount":         2000,
				"signature":      "0x14",
			},
			// Track 5: 1 purchase
			{
				"buyer_user_id":  6,
				"seller_user_id": 5,
				"content_id":     5,
				"content_type":   "track",
				"amount":         1000,
				"signature":      "0x15",
			},
			//Album 5: 1 purchase
			{
				"buyer_user_id":  6,
				"seller_user_id": 5,
				"content_id":     5,
				"content_type":   "album",
				"amount":         1000,
				"signature":      "0x16",
			},
			// Track 3 (deleted): 1 purchase
			{
				"buyer_user_id":  6,
				"seller_user_id": 3,
				"content_id":     3,
				"content_type":   "track",
				"amount":         1000,
				"signature":      "0x17",
			},
			// Track 4 (unlisted): 1 purchase
			{
				"buyer_user_id":  7,
				"seller_user_id": 4,
				"content_id":     4,
				"content_type":   "track",
				"amount":         1000,
				"signature":      "0x18",
			},
			// Album 3 (deleted): 1 purchase
			{
				"buyer_user_id":  6,
				"seller_user_id": 3,
				"content_id":     3,
				"content_type":   "album",
				"amount":         2000,
				"signature":      "0x19",
			},
			// Album 4 (unlisted): 1 purchase
			{
				"buyer_user_id":  7,
				"seller_user_id": 4,
				"content_id":     4,
				"content_type":   "album",
				"amount":         2000,
				"signature":      "0x20",
			},
			// Track 6 (purchases from before cutoff): 5 purchases
			{
				"buyer_user_id":  6,
				"seller_user_id": 6,
				"content_id":     6,
				"content_type":   "track",
				"amount":         1000,
				"created_at":     time.Now().AddDate(0, -7, 0),
				"signature":      "0x21",
			},
			{
				"buyer_user_id":  7,
				"seller_user_id": 6,
				"content_id":     6,
				"content_type":   "track",
				"amount":         1000,
				"created_at":     time.Now().AddDate(0, -7, 0),
				"signature":      "0x22",
			},
			{
				"buyer_user_id":  8,
				"seller_user_id": 6,
				"content_id":     6,
				"content_type":   "track",
				"amount":         1000,
				"created_at":     time.Now().AddDate(0, -7, 0),
				"signature":      "0x23",
			},
			{
				"buyer_user_id":  9,
				"seller_user_id": 6,
				"content_id":     6,
				"content_type":   "track",
				"amount":         1000,
				"created_at":     time.Now().AddDate(0, -7, 0),
				"signature":      "0x24",
			},
			{
				"buyer_user_id":  10,
				"seller_user_id": 6,
				"content_id":     6,
				"content_type":   "track",
				"amount":         1000,
				"created_at":     time.Now().AddDate(0, -7, 0),
				"signature":      "0x25",
			},
		},
	}

	createFixtures(app, fixtures)

	{
		var BestSellingResponse struct {
			Data    []BestSellingItem
			Related struct {
				Tracks    []dbv1.FullTrack
				Playlists []dbv1.FullPlaylist
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
