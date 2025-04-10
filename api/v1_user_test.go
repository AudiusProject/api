package api

import (
	"strings"
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetUser(t *testing.T) {
	var userResponse struct {
		Data []dbv1.FullUser
	}

	status, body := testGet(t, "/v1/full/users/7eP5n", &userResponse)
	assert.Equal(t, 200, status)

	// body is response json
	assert.True(t, strings.Contains(string(body), `"handle":"rayjacobson"`))
	assert.True(t, strings.Contains(string(body), `"user_id":1`))
	assert.True(t, strings.Contains(string(body), `"id":"7eP5n"`))

	// but we also unmarshaled into userResponse
	// for structured testing
	assert.Equal(t, userResponse.Data[0].ID, "7eP5n")
}
