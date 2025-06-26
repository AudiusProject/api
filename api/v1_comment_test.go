package api

import (
	"testing"

	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1Comment(t *testing.T) {
	app := testAppWithFixtures(t)

	{
		status, body := testGet(t, app, "/v1/comments/"+trashid.MustEncodeHashID(1))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.0.id":        trashid.MustEncodeHashID(1),
			"data.0.message":   "flame emoji",
			"data.0.user_id":   trashid.MustEncodeHashID(1),
			"related.users.#":  1,
			"related.tracks.#": 1,

			// Make sure that only one comment is returned
			"data.#":    1,
			"data.1.id": "",
		})
	}
}
