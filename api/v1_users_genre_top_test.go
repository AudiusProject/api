package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestUsersGenreTop(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "electro1", "name": "Electro 1"},
			{"user_id": 2, "handle": "hiphop1", "name": "HipHop 1"},
			{"user_id": 3, "handle": "electro2", "name": "Electro 2"},
			{"user_id": 4, "handle": "electro-no-tracks", "name": "Electro No Tracks"},
		},
		"aggregate_user": []map[string]any{
			{"user_id": 1, "follower_count": 200, "track_count": 5, "dominant_genre": "Electronic"},
			{"user_id": 2, "follower_count": 300, "track_count": 3, "dominant_genre": "Hip-Hop"},
			{"user_id": 3, "follower_count": 100, "track_count": 2, "dominant_genre": "Electronic"},
			{"user_id": 4, "follower_count": 500, "track_count": 0, "dominant_genre": "Electronic"},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/users/genre/top?genre=Electronic")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":    2,
		"data.0.id": trashid.MustEncodeHashID(1),
		"data.1.id": trashid.MustEncodeHashID(3),
	})
}
