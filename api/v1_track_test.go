package api

import (
	"strings"
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetTrack(t *testing.T) {
	var trackResponse struct {
		Data dbv1.FullTrack
	}

	status, body := testGet(t, "/v1/full/tracks/eYJyn", &trackResponse)
	assert.Equal(t, 200, status)

	assert.True(t, strings.Contains(string(body), `"title":"Culca Canyon"`))
	assert.True(t, strings.Contains(string(body), `"id":"eYJyn"`))
}
