package api

import (
	"fmt"
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetUserTracks(t *testing.T) {
	app := testAppWithFixtures(t)

	var userTracksResponse struct {
		Data []dbv1.FullTrack
	}

	// Test support for handle
	status, body := testGet(t, app, "/v1/full/users/handle/usertrackstester/tracks", &userTracksResponse)

	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(703),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Remaining assertions use the user_id version of the route
	baseUrl := fmt.Sprintf("/v1/full/users/%s/tracks", trashid.MustEncodeHashID(500))

	status, body = testGet(t, app, baseUrl, &userTracksResponse)
	assert.Equal(t, 200, status)

	// Note: Date sorts prefer release_date but fall back to created_at
	// Default sort by legacy date desc - artist pick (701) should be first
	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(703),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Sort by date asc - artist pick (701) should be first
	url := fmt.Sprintf("%s?sort=date&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(700),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Release date desc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=release_date&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(703),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Release date asc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=release_date&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(700),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Sort by legacy plays desc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort=plays&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(700),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Sort by legacy plays asc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort=plays&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(703),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Sort by sort_method plays desc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=plays&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(700),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Sort by sort_method plays asc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=plays&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(703),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Sort by title desc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=title&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first (title: Track 4)
		"data.1.id": trashid.MustEncodeHashID(702), // Track 3
		"data.2.id": trashid.MustEncodeHashID(703), // Track 2
		"data.3.id": trashid.MustEncodeHashID(700), // Track 1
	})

	// Sort by title asc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=title&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first (title: Track 4)
		"data.1.id": trashid.MustEncodeHashID(700), // Track 1
		"data.2.id": trashid.MustEncodeHashID(703), // Track 2
		"data.3.id": trashid.MustEncodeHashID(702), // Track 3
	})

	// Sort by reposts desc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=reposts&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first (75 reposts)
		"data.1.id": trashid.MustEncodeHashID(702), // 100 reposts
		"data.2.id": trashid.MustEncodeHashID(700), // 50 reposts
		"data.3.id": trashid.MustEncodeHashID(703), // 25 reposts
	})

	// Sort by reposts asc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=reposts&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first (75 reposts)
		"data.1.id": trashid.MustEncodeHashID(703), // 25 reposts
		"data.2.id": trashid.MustEncodeHashID(700), // 50 reposts
		"data.3.id": trashid.MustEncodeHashID(702), // 100 reposts
	})

	// Sort by saves desc - artist pick (701) should be first
	url = fmt.Sprintf("%s?sort_method=saves&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first (100 saves)
		"data.1.id": trashid.MustEncodeHashID(702), // 75 saves
		"data.2.id": trashid.MustEncodeHashID(700), // 50 saves
		"data.3.id": trashid.MustEncodeHashID(703), // 25 saves
	})

	// Sort by saves asc
	url = fmt.Sprintf("%s?sort_method=saves&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
		"data.1.id": trashid.MustEncodeHashID(703),
		"data.2.id": trashid.MustEncodeHashID(700),
		"data.3.id": trashid.MustEncodeHashID(702),
	})

	// Test artist pick is always first regardless of sort
	// Artist pick should be first even with title desc sort
	url = fmt.Sprintf("%s?sort_method=title&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first (track 4)
	})

	// Artist pick should be first even with plays desc sort
	url = fmt.Sprintf("%s?sort=plays&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701), // Artist pick first
	})

}

func TestGetUserTracksInvalidParams(t *testing.T) {
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
