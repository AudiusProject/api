package api

import (
	"context"
	"testing"

	"api.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTracksEndpoint(t *testing.T) {
	app := testAppWithFixtures(t)
	var resp struct {
		Data []dbv1.Track
	}

	status, body := testGet(t, app, "/v1/full/tracks?id=eYZmn", &resp)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":    "eYZmn",
		"data.0.title": "T1",
	})
}

func TestGetTracksByPermalink(t *testing.T) {
	app := testAppWithFixtures(t)

	status, body := testGet(t, app, "/v1/full/tracks?permalink=/TracksByPermalink/track-by-permalink")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.id":    "eYake",
		"data.0.title": "track by permalink",
	})
}

func TestGetTracksExcludesAccessAuthorities(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	// Track 100 has title "T1" and is returned as id eYZmn. Set access_authorities so it is gated.
	_, err := app.writePool.Exec(ctx, `UPDATE tracks SET access_authorities = ARRAY['0xgate']::text[] WHERE track_id = 100 AND is_current = true`)
	require.NoError(t, err)

	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/full/tracks?id=eYZmn", &resp)
	assert.Equal(t, 200, status)
	assert.Len(t, resp.Data, 0, "tracks with access_authorities must not be returned")
}
