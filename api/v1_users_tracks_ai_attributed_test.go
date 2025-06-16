package api

import (
	"fmt"
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetUserTracksAiAttributed(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := FixtureMap{
		"users": {
			{
				"user_id":   1,
				"handle":    "testuser1",
				"handle_lc": "testuser1",
			},
			{
				"user_id":   2,
				"handle":    "testuser2",
				"handle_lc": "testuser2",
			},
			{
				"user_id":   3,
				"handle":    "testuser3",
				"handle_lc": "testuser3",
			},
		},
		"tracks": {
			{
				"track_id": 1,
				"owner_id": 1,
			},
			{
				"track_id":               2,
				"owner_id":               2,
				"ai_attribution_user_id": 1,
				"title":                  "Track 1",
				"created_at":             parseTime(t, "2021-01-01"),
			},
			{
				"track_id":               3,
				"owner_id":               3,
				"ai_attribution_user_id": 1,
				"title":                  "Track 4",
				"created_at":             parseTime(t, "2021-01-03"),
			},
			{
				"track_id":               4,
				"owner_id":               2,
				"ai_attribution_user_id": 1,
				"title":                  "Track 3",
				"created_at":             parseTime(t, "2021-01-04"),
			},
			{
				"track_id":               5,
				"owner_id":               3,
				"ai_attribution_user_id": 1,
				"title":                  "Track 2",
				"created_at":             parseTime(t, "2021-01-02"),
				// created before other tracks but later release date
				"release_date": parseTime(t, "2021-01-05"),
			},
		},
		"aggregate_plays": {
			{
				"play_item_id": 2,
				"count":        100,
			},
			{
				"play_item_id": 3,
				"count":        50,
			},
			{
				"play_item_id": 4,
				"count":        20,
			},
			{
				"play_item_id": 5,
				"count":        10,
			},
		},
		"aggregate_track": {
			{
				"track_id":     2,
				"repost_count": 50,
				"save_count":   50,
			},
			{
				"track_id":     3,
				"repost_count": 75,
				"save_count":   100,
			},
			{
				"track_id":     4,
				"repost_count": 100,
				"save_count":   75,
			},
			{
				"track_id":     5,
				"repost_count": 25,
				"save_count":   25,
			},
		},
	}

	createFixtures(app, fixtures)

	var userTracksResponse struct {
		Data []dbv1.FullTrack
	}

	baseUrl := fmt.Sprintf("/v1/full/users/handle/testuser1/tracks/ai_attributed")

	// First test uses marshaling struct to verify that works
	status, body := testGet(t, app, baseUrl, &userTracksResponse)
	assert.Equal(t, 200, status)

	// Note: Date sorts prefer release_date but fall back to created_at
	// Default sort by legacy date desc
	jsonAssert(t, body, map[string]any{
		"data.#":    4,
		"data.0.id": trashid.MustEncodeHashID(5),
		"data.1.id": trashid.MustEncodeHashID(4),
		"data.2.id": trashid.MustEncodeHashID(3),
		"data.3.id": trashid.MustEncodeHashID(2),
	})

	// Sort by date asc
	url := fmt.Sprintf("%s?sort=date&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(2),
		"data.1.id": trashid.MustEncodeHashID(3),
		"data.2.id": trashid.MustEncodeHashID(4),
		"data.3.id": trashid.MustEncodeHashID(5),
	})

	// Release date desc
	url = fmt.Sprintf("%s?sort_method=release_date&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(5),
		"data.1.id": trashid.MustEncodeHashID(4),
		"data.2.id": trashid.MustEncodeHashID(3),
		"data.3.id": trashid.MustEncodeHashID(2),
	})

	// Release date asc
	url = fmt.Sprintf("%s?sort_method=release_date&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(2),
		"data.1.id": trashid.MustEncodeHashID(3),
		"data.2.id": trashid.MustEncodeHashID(4),
		"data.3.id": trashid.MustEncodeHashID(5),
	})

	// Sort by legacy plays desc
	url = fmt.Sprintf("%s?sort=plays&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(2),
		"data.1.id": trashid.MustEncodeHashID(3),
		"data.2.id": trashid.MustEncodeHashID(4),
		"data.3.id": trashid.MustEncodeHashID(5),
	})

	// Sort by legacy plays asc
	url = fmt.Sprintf("%s?sort=plays&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(5),
		"data.1.id": trashid.MustEncodeHashID(4),
		"data.2.id": trashid.MustEncodeHashID(3),
		"data.3.id": trashid.MustEncodeHashID(2),
	})

	// Sort by sort_method plays desc
	url = fmt.Sprintf("%s?sort_method=plays&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(2),
		"data.1.id": trashid.MustEncodeHashID(3),
		"data.2.id": trashid.MustEncodeHashID(4),
		"data.3.id": trashid.MustEncodeHashID(5),
	})

	// Sort by sort_method plays asc
	url = fmt.Sprintf("%s?sort_method=plays&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(5),
		"data.1.id": trashid.MustEncodeHashID(4),
		"data.2.id": trashid.MustEncodeHashID(3),
		"data.3.id": trashid.MustEncodeHashID(2),
	})

	// Sort by title desc
	url = fmt.Sprintf("%s?sort_method=title&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(3),
		"data.1.id": trashid.MustEncodeHashID(4),
		"data.2.id": trashid.MustEncodeHashID(5),
		"data.3.id": trashid.MustEncodeHashID(2),
	})

	// Sort by title asc
	url = fmt.Sprintf("%s?sort_method=title&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(2),
		"data.1.id": trashid.MustEncodeHashID(5),
		"data.2.id": trashid.MustEncodeHashID(4),
		"data.3.id": trashid.MustEncodeHashID(3),
	})

	// Sort by reposts desc
	url = fmt.Sprintf("%s?sort_method=reposts&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(4),
		"data.1.id": trashid.MustEncodeHashID(3),
		"data.2.id": trashid.MustEncodeHashID(2),
		"data.3.id": trashid.MustEncodeHashID(5),
	})

	// Sort by reposts asc
	url = fmt.Sprintf("%s?sort_method=reposts&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(5),
		"data.1.id": trashid.MustEncodeHashID(2),
		"data.2.id": trashid.MustEncodeHashID(3),
		"data.3.id": trashid.MustEncodeHashID(4),
	})

	// Sort by saves desc
	url = fmt.Sprintf("%s?sort_method=saves&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(3),
		"data.1.id": trashid.MustEncodeHashID(4),
		"data.2.id": trashid.MustEncodeHashID(2),
		"data.3.id": trashid.MustEncodeHashID(5),
	})

	// Sort by saves asc
	url = fmt.Sprintf("%s?sort_method=saves&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(5),
		"data.1.id": trashid.MustEncodeHashID(2),
		"data.2.id": trashid.MustEncodeHashID(4),
		"data.3.id": trashid.MustEncodeHashID(3),
	})

}

func TestGetUserTracksAiAttributedInvalidParams(t *testing.T) {
	app := testAppWithFixtures(t)
	baseUrl := fmt.Sprintf("/v1/full/users/%s/tracks", trashid.MustEncodeHashID(500))
	// Test invalid sort_method
	url := fmt.Sprintf("%s?sort_method=invalid&sort_direction=desc", baseUrl)
	status, _ := testGet(t, app, url)
	assert.Equal(t, 400, status)

	// Test invalid sort_direction
	url = fmt.Sprintf("%s?sort_method=plays&sort_direction=invalid", baseUrl)
	status, _ = testGet(t, app, url)
	assert.Equal(t, 400, status)

	// Test invalid sort
	url = fmt.Sprintf("%s?sort=invalid", baseUrl)
	status, _ = testGet(t, app, url)
	assert.Equal(t, 400, status)

	// Test invalid limit
	url = fmt.Sprintf("%s?limit=101", baseUrl)
	status, _ = testGet(t, app, url)
	assert.Equal(t, 400, status)

	// Test invalid offset
	url = fmt.Sprintf("%s?offset=invalid", baseUrl)
	status, _ = testGet(t, app, url)
	assert.Equal(t, 400, status)
}
