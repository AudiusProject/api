package api

import (
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestTrackComments(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id":   1,
				"handle":    "testuser",
				"handle_lc": "testuser",
				"name":      "Test User",
				"wallet":    "0x7d273271690538cf855e5b3002a0dd8c154bb060",
			},
		},
		"tracks": {
			{
				"track_id": 101,
				"owner_id": 1,
				"title":    "Test Track",
			},
		},
		"comments": {
			{
				"comment_id": 1,
				"user_id":    1,
				"entity_id":  101,
				"text":       "flame emoji",
			},
			{
				"comment_id": 2,
				"user_id":    2,
				"entity_id":  101,
				"text":       "thanks for the emoji",
			},
		},
		"comment_threads": {
			{
				"comment_id":        2,
				"parent_comment_id": 1,
			},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)
	status, body := testGet(t, app, "/v1/tracks/ePWJD/comments")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.message":           "flame emoji",
		"data.0.id":                "7eP5n",
		"data.0.user_id":           "7eP5n",
		"data.0.entity_id":         "ePWJD",
		"data.0.reply_count":       1,
		"data.0.replies.0.user_id": "ML51L",

		// there is no second comment
		"data.#":    1,
		"data.1.id": "",
	})
}

func TestTrackComments_UnlistedTrack(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id":   1,
				"handle":    "testuser",
				"handle_lc": "testuser",
				"name":      "Test User",
				"wallet":    "0x7d273271690538cf855e5b3002a0dd8c154bb060",
			},
		},
		"tracks": {
			{
				"track_id":    101,
				"owner_id":    1,
				"title":       "Test Track",
				"is_unlisted": true,
			},
		},
		"comments": {
			{
				"comment_id": 1,
				"user_id":    1,
				"entity_id":  101,
				"text":       "flame emoji",
			},
			{
				"comment_id": 2,
				"user_id":    2,
				"entity_id":  101,
				"text":       "thanks for the emoji",
			},
		},
		"comment_threads": {
			{
				"comment_id":        2,
				"parent_comment_id": 1,
			},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)
	status, body := testGet(t, app, "/v1/tracks/ePWJD/comments")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.message":           "flame emoji",
		"data.0.id":                "7eP5n",
		"data.0.user_id":           "7eP5n",
		"data.0.entity_id":         "ePWJD",
		"data.0.reply_count":       1,
		"data.0.replies.0.user_id": "ML51L",

		// there is no second comment
		"data.#":    1,
		"data.1.id": "",
	})
}
