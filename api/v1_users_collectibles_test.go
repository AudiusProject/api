package api

import (
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestUsersCollectibles(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id": 1,
			},
		},
		"collectibles": {
			{
				"user_id": 1,
				"data": map[string]any{
					"1:::0x1234": map[string]any{},
					"order": []string{
						"1:::0x1234",
					},
				},
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/collectibles")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.1:::0x1234": "{}",
		"data.order.#":    1,
		"data.order.0":    "1:::0x1234",
	})
}
