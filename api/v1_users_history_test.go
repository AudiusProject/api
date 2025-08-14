package api

import (
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
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

func TestUserListeningHistoryUnlisted(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id": 1,
				"handle":  "user1",
			},
			{
				"user_id": 2,
				"handle":  "user2",
				"wallet":  "0x7d273271690538cf855e5b3002a0dd8c154bb060",
			},
		},
		"tracks": []map[string]any{
			{
				"track_id":    1,
				"title":       "Public Track",
				"owner_id":    1,
				"is_unlisted": false,
				"created_at":  parseTime(t, "2024-01-01"),
			},
			{
				"track_id":    2,
				"title":       "Unlisted Track",
				"owner_id":    1,
				"is_unlisted": true,
				"created_at":  parseTime(t, "2024-01-01"),
			},
		},
		"user_listening_history": []map[string]any{
			{
				"user_id": 2,
				"listening_history": []map[string]any{
					{
						"track_id":   1,
						"play_count": 1,
						"timestamp":  parseTime(t, "2024-01-01"),
					},
					{
						"track_id":   2,
						"play_count": 1,
						"timestamp":  parseTime(t, "2024-01-01"),
					},
				},
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)
	user2Id := trashid.MustEncodeHashID(2)

	{
		status, body := testGetWithWallet(t, app, "/v1/full/users/"+user2Id+"/history/tracks?user_id="+user2Id, "0x7d273271690538cf855e5b3002a0dd8c154bb060")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":         2,
			"data.0.item_id": 1,
			"data.1.item_id": 2,
		})
	}

	{
		status, body := testGet(t, app, "/v1/full/users/"+user2Id+"/history/tracks")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":         1,
			"data.0.item_id": 1,
		})
	}
}
