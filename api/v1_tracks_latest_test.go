package api

import (
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetLatest(t *testing.T) {
	app := testAppWithFixtures(t)
	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/tracks/latest?limit=5", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 5, len(resp.Data))
}

func TestGetLatestWithGenre(t *testing.T) {
	app := testAppWithFixtures(t)
	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/tracks/latest?genre=LatestTestGenreA", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(resp.Data))
	assert.Equal(t, trashid.MustEncodeHashID(801), resp.Data[0].ID)
	assert.Equal(t, trashid.MustEncodeHashID(800), resp.Data[1].ID)
	for _, track := range resp.Data {
		assert.Equal(t, "LatestTestGenreA", track.Genre.String)
	}
}

func TestGetLatestWithLimitOffset(t *testing.T) {
	app := testAppWithFixtures(t)
	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/tracks/latest?genre=LatestTestGenreA&limit=1&offset=0", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, trashid.MustEncodeHashID(801), resp.Data[0].ID)

	status, _ = testGet(t, app, "/v1/tracks/latest?genre=LatestTestGenreA&limit=1&offset=1", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, trashid.MustEncodeHashID(800), resp.Data[0].ID)
}

func TestGetLatestExcludesUnlisted(t *testing.T) {
	app := testAppWithFixtures(t)
	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/tracks/latest?genre=LatestTestGenreB", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, trashid.MustEncodeHashID(803), resp.Data[0].ID)
}
