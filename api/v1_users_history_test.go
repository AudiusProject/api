package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserListeningHistory(t *testing.T) {
	app := testAppWithFixtures(t)
	// Test in-order history (full)
	status, body := testGet(t, app, "/v1/full/users/DNRpD/history/tracks")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.class":     "track_activity_full",
		"data.0.item_type": "track",
		"data.0.item.id":   "eYZmn",
		"data.1.item.id":   "ePWJD",
		"data.2.item.id":   "eJpoL",
	})

	// Test reverse history (non-full)
	status2, body2 := testGet(t, app, "/v1/users/DNRpD/history/tracks?sort_direction=asc")
	assert.Equal(t, 200, status2)
	jsonAssert(t, body2, map[string]any{
		"data.0.class":   "track_activity",
		"data.0.item.id": "eJpoL",
		"data.1.item.id": "ePWJD",
		"data.2.item.id": "eYZmn",
	})

	// Test sorting by user plays count
	status3, body3 := testGet(t, app, "/v1/users/DNRpD/history/tracks?sort_method=most_listens_by_user")
	assert.Equal(t, 200, status3)
	jsonAssert(t, body3, map[string]any{
		"data.0.item.id": "eJpoL",
		"data.1.item.id": "eYZmn",
		"data.2.item.id": "ePWJD",
	})

	// Test sorting by title
	status4, body4 := testGet(t, app, "/v1/users/DNRpD/history/tracks?sort_method=title&sort_direction=desc")
	assert.Equal(t, 200, status4)
	jsonAssert(t, body4, map[string]any{
		"data.0.item.title": "Trending Gated Jazz Track 1",
		"data.1.item.title": "T2",
		"data.2.item.title": "T1",
	})

	// Test sorting by artist name
	status5, body5 := testGet(t, app, "/v1/users/DNRpD/history/tracks?sort_method=artist_name&sort_direction=asc")
	assert.Equal(t, 200, status5)
	jsonAssert(t, body5, map[string]any{
		"data.0.item.user.name": "Guy in Trending",
		"data.1.item.user.name": "Ray Jacobson",
		"data.2.item.user.name": "Ray Jacobson",
	})

	// Test filter
	status6, body6 := testGet(t, app, "/v1/users/DNRpD/history/tracks?query=Jazz")
	assert.Equal(t, 200, status6)
	jsonAssert(t, body6, map[string]any{
		"data.0.item.id":    "eJpoL",
		"data.0.item.title": "Trending Gated Jazz Track 1",
	})
}
