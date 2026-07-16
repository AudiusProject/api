package api

import (
	"net/http/httptest"
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrackDownload(t *testing.T) {
	app := testAppWithFixtures(t)
	req := httptest.NewRequest("GET", "/v1/tracks/eYZmn/download", nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Contains(t, res.Header.Get("Location"), "tracks/cidstream/QmT1TrackCid")
	assert.Contains(t, res.Header.Get("Location"), "signature=%7B%22data%22%3A%22%7B%5C%22cid%5C%22%3A%5C%22QmT1TrackCid%5C%22%2C%5C%22timestamp%5C%22%3")
	assert.Contains(t, res.Header.Get("Location"), "filename=T1.mp3")
}

// A downloadable track whose row has no orig_file_cid or track_cid (e.g. an
// upload-v2 row that never got its cid backfill) must 404 rather than
// redirect to a signed URL with an empty cid, which content nodes reject.
func TestGetTrackDownload_NoCid(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id":        1,
				"owner_id":        1,
				"title":           "No Cid",
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
	database.Seed(app.pool.Replicas[0], fixtures)
	req := httptest.NewRequest("GET", "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/download", nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 404, res.StatusCode)
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
	database.Seed(app.pool.Replicas[0], fixtures)
	req := httptest.NewRequest("GET", "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/download", nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Contains(t, res.Header.Get("Location"), "signature=%7B%22data%22%3A%22%7B%5C%22cid%5C%22%3A%5C%22QmX123%5C%22%2C%5C%22timestamp%5C%22%3")
	assert.Contains(t, res.Header.Get("Location"), "filename=DharitRocks.wav")
}
