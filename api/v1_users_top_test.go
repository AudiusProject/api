package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestUsersTop(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "user1", "name": "User 1"},
			{"user_id": 2, "handle": "user2", "name": "User 2"},
			{"user_id": 3, "handle": "user3", "name": "User 3"},
			{"user_id": 4, "handle": "user4", "name": "User 4"},
		},
		"aggregate_user": []map[string]any{
			{"user_id": 1, "track_count": 1, "follower_count": 10},
			{"user_id": 2, "track_count": 1, "follower_count": 100},
			{"user_id": 3, "track_count": 2, "follower_count": 100},
			{"user_id": 4, "track_count": 0, "follower_count": 1000},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	{
		status, body := testGet(t, app, "/v1/users/top?limit=2")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":    2,
			"data.0.id": trashid.MustEncodeHashID(2),
			"data.1.id": trashid.MustEncodeHashID(3),
		})
	}

	{
		status, body := testGet(t, app, "/v1/users/top?limit=2&offset=1")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":    2,
			"data.0.id": trashid.MustEncodeHashID(3),
			"data.1.id": trashid.MustEncodeHashID(1),
		})
	}
}
