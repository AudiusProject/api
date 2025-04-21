package api

import (
	"strings"
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetTrack(t *testing.T) {
	app := fixturesTestApp(t)

	var trackResponse struct {
		Data dbv1.FullTrack
	}

	status, body := testGet(t, app, "/v1/full/tracks/eYJyn", &trackResponse)
	assert.Equal(t, 200, status)

	assert.True(t, strings.Contains(string(body), `"title":"Culca Canyon"`))
	assert.True(t, strings.Contains(string(body), `"id":"eYJyn"`))
}

func TestGetTrackFollowDownloadAcess(t *testing.T) {
	app := fixturesTestApp(t)

	var trackResponse struct {
		Data dbv1.FullTrack
	}
	// No access
	_, body1 := testGet(t, app, "/v1/full/tracks/eYRWn", &trackResponse)
	assert.True(t, strings.Contains(string(body1), `"title":"Follow Gated Download"`))
	assert.True(t, strings.Contains(string(body1), `"access":{"stream":true,"download":false}`))

	// With access
	_, body2 := testGet(t, app, "/v1/full/tracks/eYRWn?user_id=ELKzn", &trackResponse)
	assert.True(t, strings.Contains(string(body2), `"title":"Follow Gated Download"`))
	assert.True(t, strings.Contains(string(body2), `"access":{"stream":true,"download":true}`))
}

func TestGetTrackTipStreamAccess(t *testing.T) {
	app := fixturesTestApp(t)

	var trackResponse struct {
		Data dbv1.FullTrack
	}
	// No access
	_, body1 := testGet(t, app, "/v1/full/tracks/L5x7n", &trackResponse)
	assert.True(t, strings.Contains(string(body1), `"title":"Tip Gated Stream"`))
	assert.True(t, strings.Contains(string(body1), `"access":{"stream":false,"download":false}`))

	// With access
	_, body2 := testGet(t, app, "/v1/full/tracks/L5x7n?user_id=ELKzn", &trackResponse)
	assert.True(t, strings.Contains(string(body2), `"title":"Tip Gated Stream"`))
	assert.True(t, strings.Contains(string(body2), `"access":{"stream":true,"download":false}`))
}
