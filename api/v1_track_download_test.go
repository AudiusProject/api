package api

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTrackDownload(t *testing.T) {
	req := httptest.NewRequest("GET", "/v1/tracks/eYZmn/download", nil)
	res, err := app.Test(req, -1)
	assert.NoError(t, err)
	assert.Contains(t, res.Header.Get("Location"), "https://dummynode.com/tracks/cidstream/?signature=%7B%22data%22%3A%22%7B%5C%22cid%5C%22%3A%5C%22%5C%22%2C%5C%22timestamp%5C%22%3")
}
