package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetUsersHandle(t *testing.T) {
	var accountResponse struct {
		Data dbv1.FullAccount
	}
	status, body := testGet(t, "/v1/users/handle/rayjacobson", &accountResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.id":     "7eP5n",
		"data.handle": "rayjacobson",
	})
}
