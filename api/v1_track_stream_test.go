package api

import (
	"net/http/httptest"
	"testing"

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
