package api

import (
	"testing"
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1TracksRecentComments(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := FixtureMap{
		"users": []map[string]any{
			{
				"user_id": 1,
				"handle":  "user1",
				"name":    "User 1",
			},
			{
				"user_id": 2,
				"handle":  "user2",
				"name":    "User 2",
			},
			{
				"user_id": 3,
				"handle":  "user3",
				"name":    "User 3",
			},
		},
		"tracks": []map[string]any{
			{
				"track_id":   1,
				"owner_id":   1,
				"created_at": time.Now().Add(-time.Duration(1) * time.Hour),
			},
			{
				"track_id":   2,
				"owner_id":   1,
				"created_at": time.Now().Add(-time.Duration(2) * time.Hour),
			},
			{
				"track_id":   3,
				"owner_id":   2,
				"created_at": time.Now().Add(-time.Duration(3) * time.Hour),
			},
		},
		"comments": []map[string]any{
			{
				"comment_id":  3,
				"entity_id":   2,
				"entity_type": "Track",
				"user_id":     3,
				"created_at":  time.Now().Add(-time.Duration(1) * time.Hour),
			},
			{
				"comment_id":  2,
				"entity_id":   2,
				"entity_type": "Track",
				"user_id":     1,
				"created_at":  time.Now().Add(-time.Duration(2) * time.Hour),
			},
			{
				"comment_id":  1,
				"entity_id":   1,
				"entity_type": "Track",
				"user_id":     3,
				"created_at":  time.Now().Add(-time.Duration(3) * time.Hour),
			},
		},
	}

	createFixtures(app, fixtures)

	{
		status, body := testGet(t, app, "/v1/full/tracks/recent-comments")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":    2,
			"data.0.id": trashid.MustEncodeHashID(2),
			"data.1.id": trashid.MustEncodeHashID(1),
		})
	}
}

func TestV1TracksRecentCommentsInvalidParams(t *testing.T) {
	app := emptyTestApp(t)

	for _, val := range []string{"-1", "101", "invalid"} {
		status, _ := testGet(t, app, "/v1/full/tracks/recent-comments?limit="+val)
		assert.Equal(t, 400, status)
	}

	for _, val := range []string{"-1", "invalid"} {
		status, _ := testGet(t, app, "/v1/full/tracks/recent-comments?offset="+val)
		assert.Equal(t, 400, status)
	}
}
