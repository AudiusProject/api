package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetUser(t *testing.T) {
	app := testAppWithFixtures(t)
	var userResponse struct {
		Data []dbv1.FullUser
	}

	status, body := testGet(t, app, "/v1/full/users/7eP5n", &userResponse)
	assert.Equal(t, 200, status)

	// body is response json
	jsonAssert(t, body, map[string]any{
		"data.0.handle":  "rayjacobson",
		"data.0.user_id": 1,
		"data.0.id":      "7eP5n",
	})

	// but we also unmarshaled into userResponse
	// for structured testing
	assert.Equal(t, userResponse.Data[0].ID, trashid.HashId(1))
}
