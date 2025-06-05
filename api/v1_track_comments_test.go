package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTrackComments(t *testing.T) {
	app := testAppWithFixtures(t)
	status, body := testGet(t, app, "/v1/tracks/ePgRD/comments")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.message":           "flame emoji",
		"data.0.id":                "7eP5n",
		"data.0.user_id":           "7eP5n",
		"data.0.entity_id":         "ePgRD",
		"data.0.reply_count":       1,
		"data.0.replies.0.user_id": "ML51L",

		// there is no second comment
		"data.#":    1,
		"data.1.id": "",
	})
}
