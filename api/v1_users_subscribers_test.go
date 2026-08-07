package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestUsersSubscribers(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "artist", "name": "Artist"},
			{"user_id": 2, "handle": "subscriber2", "name": "Subscriber 2"},
			{"user_id": 3, "handle": "subscriber3", "name": "Subscriber 3"},
			{"user_id": 4, "handle": "deletedsub", "name": "Deleted Sub"},
			{"user_id": 5, "handle": "oldsub", "name": "Old Sub"},
			{"user_id": 6, "handle": "eventonlyfan", "name": "Event Only Fan"},
		},
		"aggregate_user": []map[string]any{
			{"user_id": 1, "track_count": 1},
			{"user_id": 2, "track_count": 1},
			{"user_id": 3, "track_count": 1},
			{"user_id": 4, "track_count": 1},
			{"user_id": 5, "track_count": 1},
			{"user_id": 6, "track_count": 1},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)
	// The last row is an Event subscription whose event id collides with the
	// artist's user id (user_id mirrors the event id for Event rows). Its
	// subscriber follows event 1, not user 1, and must NOT be listed.
	_, err := app.pool.Exec(t.Context(), `
		INSERT INTO subscriptions (user_id, subscriber_id, is_current, is_delete, txhash, entity_type, entity_id)
		VALUES
			(1, 3, TRUE, FALSE, 'tx-sub-3', 'User', NULL),
			(1, 2, TRUE, FALSE, 'tx-sub-2', 'User', NULL),
			(1, 4, TRUE, TRUE, 'tx-sub-deleted', 'User', NULL),
			(1, 5, FALSE, FALSE, 'tx-sub-not-current', 'User', NULL),
			(1, 6, TRUE, FALSE, 'tx-sub-event-collision', 'Event', 1)
	`)
	assert.NoError(t, err)

	artistID := trashid.MustEncodeHashID(1)
	status, body := testGet(t, app, "/v1/users/"+artistID+"/subscribers")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":    2,
		"data.0.id": trashid.MustEncodeHashID(2),
		"data.1.id": trashid.MustEncodeHashID(3),
	})
}
