package api

import (
	"strings"
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetUser(t *testing.T) {
	app := fixturesTestApp(t)

	var userResponse struct {
		Data []dbv1.FullUser
	}

	status, body := testGet(t, app, "/v1/full/users/7eP5n", &userResponse)
	assert.Equal(t, 200, status)

	// body is response json
	assert.True(t, strings.Contains(string(body), `"handle":"rayjacobson"`))
	assert.True(t, strings.Contains(string(body), `"user_id":1`))
	assert.True(t, strings.Contains(string(body), `"id":"7eP5n"`))

	// but we also unmarshaled into userResponse
	// for structured testing
	assert.Equal(t, userResponse.Data[0].ID, trashid.HashId(1))
}
