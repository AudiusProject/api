package api

import (
	"net/http/httptest"
	"testing"

	"api.audius.co/api/testdata"
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

const (
	ownerWallet    = "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	strangerWallet = "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"
	managerWallet  = "0x4954d18926ba0ed9378938444731be4e622537b2"
)

// seedNonDownloadableTrack sets up the shape most tracks on the network have:
// an artist who never turned downloads on, whose original upload is still
// sitting on the content node.
func seedNonDownloadableTrack(t *testing.T) *ApiServer {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id":        1,
				"owner_id":        1,
				"title":           "Private Master",
				"track_cid":       "QmTranscode",
				"orig_file_cid":   "QmOriginal",
				"orig_filename":   "PrivateMaster.wav",
				"is_downloadable": false,
			},
		},
		"users": []map[string]any{
			{"user_id": 1, "handle": "artist", "wallet": ownerWallet},
			{"user_id": 2, "handle": "stranger", "wallet": strangerWallet},
			{"user_id": 3, "handle": "manager", "wallet": managerWallet},
		},
	})
	return app
}

func downloadWithWallet(t *testing.T, app *ApiServer, path string, wallet string) (int, string) {
	req := httptest.NewRequest("GET", path, nil)
	if wallet != "" {
		sigData := testdata.GetSignatureData(wallet)
		req.Header.Set("Encoded-Data-Message", sigData.Message)
		req.Header.Set("Encoded-Data-Signature", sigData.Signature)
	}
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	return res.StatusCode, res.Header.Get("Location")
}

// An artist must be able to get their own upload back even with downloads off
// for everyone else - that is what the edit page's "Download File" button does.
func TestGetTrackDownload_OwnerOfNonDownloadableTrack(t *testing.T) {
	app := seedNonDownloadableTrack(t)
	path := "/v1/tracks/" + trashid.MustEncodeHashID(1) + "/download"

	status, location := downloadWithWallet(t, app, path, ownerWallet)
	assert.Equal(t, 302, status)
	assert.Contains(t, location, "tracks/cidstream/QmOriginal")
	assert.Contains(t, location, "filename=PrivateMaster.wav")
}

// A manager the artist granted access to acts for them here too.
func TestGetTrackDownload_ManagerOfNonDownloadableTrack(t *testing.T) {
	app := seedNonDownloadableTrack(t)
	database.SeedTable(app.pool.Replicas[0], "grants", []map[string]any{
		{
			"user_id":         1,
			"grantee_address": managerWallet,
			"is_approved":     true,
			"is_revoked":      false,
		},
	})

	status, location := downloadWithWallet(t, app,
		"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/download", managerWallet)
	assert.Equal(t, 302, status)
	assert.Contains(t, location, "tracks/cidstream/QmOriginal")
}

// Everyone else still gets the 404 the artist asked for by leaving downloads
// off. Claiming to be the owner through the user_id query param must not be
// enough: the bypass keys on the recovered signature, and the auth middleware
// separately refuses a user_id no signature backs (403 rather than 404).
func TestGetTrackDownload_NonOwnerOfNonDownloadableTrack(t *testing.T) {
	app := seedNonDownloadableTrack(t)
	trackPath := "/v1/tracks/" + trashid.MustEncodeHashID(1) + "/download"
	asOwner := trackPath + "?user_id=" + trashid.MustEncodeHashID(1)

	status, _ := downloadWithWallet(t, app, trackPath, "")
	assert.Equal(t, 404, status, "anonymous")

	status, _ = downloadWithWallet(t, app, trackPath, strangerWallet)
	assert.Equal(t, 404, status, "signed by another wallet")

	status, _ = downloadWithWallet(t, app, asOwner, "")
	assert.Equal(t, 403, status, "user_id claiming to be the owner, unsigned")

	status, _ = downloadWithWallet(t, app, asOwner, strangerWallet)
	assert.Equal(t, 403, status, "user_id claiming to be the owner, signed by another wallet")
}

// With no original kept, the download serves the mp3 transcode, so the name it
// is served under has to say mp3 rather than the format that was uploaded.
func TestGetTrackDownload_FilenameFallsBackToMp3(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"tracks": []map[string]any{
			{
				"track_id":        1,
				"owner_id":        1,
				"title":           "Vol. 2",
				"track_cid":       "QmTranscode",
				"orig_filename":   "Vol. 2.wav",
				"is_downloadable": true,
			},
		},
		"users": []map[string]any{
			{"user_id": 1, "handle": "artist", "wallet": ownerWallet},
		},
	})

	status, location := downloadWithWallet(t, app,
		"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/download", "")
	assert.Equal(t, 302, status)
	assert.Contains(t, location, "tracks/cidstream/QmTranscode")
	assert.Contains(t, location, "filename=Vol.+2.mp3")
}
