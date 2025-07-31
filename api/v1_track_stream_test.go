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
	assert.Contains(t, res.Header.Get("Location"), "tracks/cidstream/?id3=true&id3_artist=&id3_title=Culca+Canyon&signature=%7B%22data%22%3A%22%7B%5C%22cid%5C%22%3A%5C%22%5C%22%2C%5C%22timestamp%5C%22%3")
}
