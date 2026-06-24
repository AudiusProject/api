package api

import (
	"fmt"
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrackStems(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id": 1,
			},
		},
		"tracks": {
			{
				"track_id": 1,
				"owner_id": 1,
			},
			{
				"track_id":      2,
				"owner_id":      1,
				"track_cid":     "testcid1",
				"blocknumber":   101,
				"orig_filename": "stem1.wav",
				"stem_of": map[string]any{
					"parent_track_id": 1,
					"category":        "bass",
				},
			},
			{
				"track_id":      3,
				"owner_id":      1,
				"track_cid":     "testcid2",
				"blocknumber":   101,
				"orig_filename": "stem2.wav",
				"stem_of": map[string]any{
					"parent_track_id": 1,
					"category":        "vocals",
				},
			},
			{
				"track_id":    4,
				"owner_id":    1,
				"title":       "Stem Without Original Filename",
				"track_cid":   "testcid3",
				"blocknumber": 101,
				"stem_of": map[string]any{
					"parent_track_id": 1,
					"category":        "guitar",
				},
			},
		},
		"stems": {
			{
				"child_track_id":  2,
				"parent_track_id": 1,
			},
			{
				"child_track_id":  3,
				"parent_track_id": 1,
			},
			{
				"child_track_id":  4,
				"parent_track_id": 1,
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, fmt.Sprintf("/v1/full/tracks/%s/stems", trashid.MustEncodeHashID(1)))
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":               3,
		"data.0.id":            trashid.MustEncodeHashID(2),
		"data.0.parent_id":     trashid.MustEncodeHashID(1),
		"data.0.category":      "bass",
		"data.0.orig_filename": "stem1.wav",
		"data.0.cid":           "testcid1",
		"data.0.user_id":       trashid.MustEncodeHashID(1),
		"data.1.id":            trashid.MustEncodeHashID(3),
		"data.1.parent_id":     trashid.MustEncodeHashID(1),
		"data.1.category":      "vocals",
		"data.1.orig_filename": "stem2.wav",
		"data.1.cid":           "testcid2",
		"data.1.user_id":       trashid.MustEncodeHashID(1),
		"data.2.id":            trashid.MustEncodeHashID(4),
		"data.2.parent_id":     trashid.MustEncodeHashID(1),
		"data.2.category":      "guitar",
		"data.2.orig_filename": "Stem Without Original Filename",
		"data.2.cid":           "testcid3",
		"data.2.user_id":       trashid.MustEncodeHashID(1),
	})
}
