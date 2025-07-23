package api

import (
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1TrackRemixing(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id": 1,
				"handle":  "user1",
			},
			{
				"user_id": 2,
				"handle":  "user2",
			},
		},
		"tracks": []map[string]any{
			{
				"track_id":        1,
				"title":           "Parent Track 1",
				"owner_id":        1,
				"is_stream_gated": false,
				"is_unlisted":     false,
				"created_at":      parseTime(t, "2024-01-01"),
			},
			{
				"track_id":        2,
				"title":           "Parent Track 2",
				"owner_id":        1,
				"is_stream_gated": false,
				"is_unlisted":     true,
				"created_at":      parseTime(t, "2024-01-01"),
			},
			{
				"track_id":        3,
				"title":           "Parent Track 3",
				"owner_id":        1,
				"is_stream_gated": false,
				"is_unlisted":     false,
				"created_at":      parseTime(t, "2024-01-01"),
			},
			{
				"track_id":        10,
				"title":           "Child Track 1",
				"owner_id":        2,
				"is_stream_gated": false,
				"is_unlisted":     false,
				"created_at":      parseTime(t, "2024-01-02"),
			},
			{
				"track_id":        11,
				"title":           "Child Track 2",
				"owner_id":        2,
				"is_stream_gated": false,
				"created_at":      parseTime(t, "2024-01-02"),
			},
			{
				"track_id":        12,
				"title":           "Child Track 3",
				"owner_id":        2,
				"is_stream_gated": true,
				"is_unlisted":     false,
				"created_at":      parseTime(t, "2024-01-02"),
			},
		},
		"remixes": []map[string]any{
			{
				"parent_track_id": 1,
				"child_track_id":  10,
			},
			{
				"parent_track_id": 3,
				"child_track_id":  10,
			},
			{
				"parent_track_id": 2,
				"child_track_id":  11,
			},
			{
				"parent_track_id": 1,
				"child_track_id":  12,
			},
		},
	}

	database.Seed(app.pool, fixtures)

	status, body := testGet(t, app, "/v1/full/tracks/"+trashid.MustEncodeHashID(10)+"/remixing")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":    2,
		"data.0.id": trashid.MustEncodeHashID(3),
		"data.1.id": trashid.MustEncodeHashID(1),
	})

	status, body = testGet(t, app, "/v1/full/tracks/"+trashid.MustEncodeHashID(11)+"/remixing")

	jsonAssert(t, body, map[string]any{
		"data.#": 0,
	})

	status, body = testGet(t, app, "/v1/full/tracks/"+trashid.MustEncodeHashID(12)+"/remixing")

	jsonAssert(t, body, map[string]any{
		"data.#": 0,
	})
}

func TestV1TrackRemixingInvalidParams(t *testing.T) {
	app := emptyTestApp(t)

	baseUrl := "/v1/full/tracks/" + trashid.MustEncodeHashID(10) + "/remixing"

	status, _ := testGet(t, app, baseUrl+"?limit=invalid")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, baseUrl+"?limit=-1")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, baseUrl+"?limit=101")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, baseUrl+"?offset=-1")
	assert.Equal(t, 400, status)

	status, _ = testGet(t, app, baseUrl+"?offset=invalid")
	assert.Equal(t, 400, status)
}
