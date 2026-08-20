package api

import (
	"net/http/httptest"
	"testing"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrackStream(t *testing.T) {
	app := testAppWithFixtures(t)
	req := httptest.NewRequest("GET", "/v1/tracks/eYJyn/stream", nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	location := res.Header.Get("Location")
	assert.Contains(t, location, "tracks/cidstream/")
	assert.Contains(t, location, "signature=")
	// ID3 tags are opt-in via ?id3=true; verify they are NOT present by default.
	assert.NotContains(t, location, "id3=true")
	assert.NotContains(t, location, "id3_artist=")
	assert.NotContains(t, location, "id3_title=")
}

// A streamable track whose row has no track_cid (e.g. an upload-v2 row that
// never got its cid backfill) must 404 rather than redirect to a signed URL
// with an empty cid, which content nodes reject.
func TestGetTrackStream_NoCid(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id": 1,
				"owner_id": 1,
				"title":    "No Cid",
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
	req := httptest.NewRequest("GET", "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/stream", nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 404, res.StatusCode)
}

func TestGetTrackStreamWithID3(t *testing.T) {
	app := testAppWithFixtures(t)
	req := httptest.NewRequest("GET", "/v1/tracks/eYJyn/stream?id3=true", nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	location := res.Header.Get("Location")
	assert.Contains(t, location, "tracks/cidstream/")
	assert.Contains(t, location, "id3=true")
	assert.Contains(t, location, "id3_title=Culca+Canyon")
}

// A track whose owner is no longer active - the artist deactivated their own
// account, or the account was delisted by the trusted notifier - reports
// is_streamable=false on the track response. The stream endpoint must refuse
// to serve the audio rather than redirecting to a signed content-node URL.
func TestGetTrackStream_DeactivatedOwner(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id":  1,
				"owner_id":  1,
				"title":     "Deactivated Owner",
				"track_cid": "QmDeactivatedOwnerCid",
			},
		},
		"users": []map[string]any{
			{
				"user_id":        1,
				"handle":         "testuser1",
				"is_deactivated": true,
			},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)
	req := httptest.NewRequest("GET", "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/stream", nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 404, res.StatusCode)
	assert.Empty(t, res.Header.Get("Location"))
}

// A deleted track is likewise non-streamable and must not redirect.
func TestGetTrackStream_DeletedTrack(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id":  1,
				"owner_id":  1,
				"title":     "Deleted",
				"track_cid": "QmDeletedTrackCid",
				"is_delete": true,
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
	req := httptest.NewRequest("GET", "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/stream", nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Equal(t, 404, res.StatusCode)
	assert.Empty(t, res.Header.Get("Location"))
}
