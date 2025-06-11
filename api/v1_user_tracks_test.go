package api

import (
	"fmt"
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
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
		"data.0.id": trashid.MustEncodeHashID(703),
		"data.1.id": trashid.MustEncodeHashID(702),
		"data.2.id": trashid.MustEncodeHashID(701),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Remaining assertions use the user_id version of the route
	baseUrl := fmt.Sprintf("/v1/full/users/%s/tracks", trashid.MustEncodeHashID(500))

	status, body = testGet(t, app, baseUrl, &userTracksResponse)
	assert.Equal(t, 200, status)

	// Note: Date sorts prefer release_date but fall back to created_at
	// Default sort by legacy date desc
	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(703),
		"data.1.id": trashid.MustEncodeHashID(702),
		"data.2.id": trashid.MustEncodeHashID(701),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Sort by date asc
	url := fmt.Sprintf("%s?sort=date&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(700),
		"data.1.id": trashid.MustEncodeHashID(701),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Release date desc
	url = fmt.Sprintf("%s?sort_method=release_date&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(703),
		"data.1.id": trashid.MustEncodeHashID(702),
		"data.2.id": trashid.MustEncodeHashID(701),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Release date asc
	url = fmt.Sprintf("%s?sort_method=release_date&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(700),
		"data.1.id": trashid.MustEncodeHashID(701),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Sort by legacy plays desc
	url = fmt.Sprintf("%s?sort=plays&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(700),
		"data.1.id": trashid.MustEncodeHashID(701),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Sort by legacy plays asc
	url = fmt.Sprintf("%s?sort=plays&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(703),
		"data.1.id": trashid.MustEncodeHashID(702),
		"data.2.id": trashid.MustEncodeHashID(701),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Sort by sort_method plays desc
	url = fmt.Sprintf("%s?sort_method=plays&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(700),
		"data.1.id": trashid.MustEncodeHashID(701),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Sort by sort_method plays asc
	url = fmt.Sprintf("%s?sort_method=plays&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(703),
		"data.1.id": trashid.MustEncodeHashID(702),
		"data.2.id": trashid.MustEncodeHashID(701),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Sort by title desc
	url = fmt.Sprintf("%s?sort_method=title&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701),
		"data.1.id": trashid.MustEncodeHashID(702),
		"data.2.id": trashid.MustEncodeHashID(703),
		"data.3.id": trashid.MustEncodeHashID(700),
	})

	// Sort by title asc
	url = fmt.Sprintf("%s?sort_method=title&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(700),
		"data.1.id": trashid.MustEncodeHashID(703),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(701),
	})

	// Sort by reposts desc
	url = fmt.Sprintf("%s?sort_method=reposts&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(702),
		"data.1.id": trashid.MustEncodeHashID(701),
		"data.2.id": trashid.MustEncodeHashID(700),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Sort by reposts asc
	url = fmt.Sprintf("%s?sort_method=reposts&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(703),
		"data.1.id": trashid.MustEncodeHashID(700),
		"data.2.id": trashid.MustEncodeHashID(701),
		"data.3.id": trashid.MustEncodeHashID(702),
	})

	// Sort by saves desc
	url = fmt.Sprintf("%s?sort_method=saves&sort_direction=desc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(701),
		"data.1.id": trashid.MustEncodeHashID(702),
		"data.2.id": trashid.MustEncodeHashID(700),
		"data.3.id": trashid.MustEncodeHashID(703),
	})

	// Sort by saves asc
	url = fmt.Sprintf("%s?sort_method=saves&sort_direction=asc", baseUrl)
	status, body = testGet(t, app, url, &userTracksResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(703),
		"data.1.id": trashid.MustEncodeHashID(700),
		"data.2.id": trashid.MustEncodeHashID(702),
		"data.3.id": trashid.MustEncodeHashID(701),
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
