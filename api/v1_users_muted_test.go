package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestUsersMuted(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "owner", "name": "Owner"},
			{"user_id": 2, "handle": "muted-user", "name": "Muted User"},
			{"user_id": 3, "handle": "unmuted-user", "name": "Unmuted User"},
		},
		"aggregate_user": []map[string]any{
			{"user_id": 1, "track_count": 1},
			{"user_id": 2, "track_count": 1},
			{"user_id": 3, "track_count": 1},
		},
		"muted_users": []map[string]any{
			{
				"user_id":       1,
				"muted_user_id": 2,
				"is_delete":     false,
				"txhash":        "tx-mute-active",
				"blockhash":     "block-mute-active",
			},
			{
				"user_id":       1,
				"muted_user_id": 3,
				"is_delete":     true,
				"txhash":        "tx-mute-deleted",
				"blockhash":     "block-mute-deleted",
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	userID := trashid.MustEncodeHashID(1)
	status, body := testGet(t, app, "/v1/users/"+userID+"/muted")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":    1,
		"data.0.id": trashid.MustEncodeHashID(2),
	})
}
