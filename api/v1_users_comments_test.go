package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUserComments(t *testing.T) {
	app := testAppWithFixtures(t)
	status, body := testGet(t, app, "/v1/users/7eP5n/comments")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.message":   "flame emoji",
		"data.0.user_id":   "7eP5n",
		"data.0.entity_id": "ePgRD",
	})
}
