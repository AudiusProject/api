package api

import (
	"net/http/httptest"
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrackDownload(t *testing.T) {
	app := testAppWithFixtures(t)
	req := httptest.NewRequest("GET", "/v1/tracks/eYZmn/download", nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Contains(t, res.Header.Get("Location"), "signature=%7B%22data%22%3A%22%7B%5C%22cid%5C%22%3A%5C%22%5C%22%2C%5C%22timestamp%5C%22%3")
	assert.Contains(t, res.Header.Get("Location"), "filename=T1.mp3")
}

func TestGetTrackDownload_Original(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id":        1,
				"owner_id":        1,
				"title":           "T1",
				"orig_file_cid":   "QmX123",
				"orig_filename":   "DharitRocks.wav",
				"is_downloadable": true,
			},
		},
		"users": []map[string]any{
			{
				"user_id": 1,
				"handle":  "testuser1",
			},
		},
	}
	database.Seed(app.pool, fixtures)
	req := httptest.NewRequest("GET", "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/download", nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Contains(t, res.Header.Get("Location"), "signature=%7B%22data%22%3A%22%7B%5C%22cid%5C%22%3A%5C%22QmX123%5C%22%2C%5C%22timestamp%5C%22%3")
	assert.Contains(t, res.Header.Get("Location"), "filename=DharitRocks.wav")
}
