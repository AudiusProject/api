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

	// Use a wallet that has test signature data
	gateWallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	// Track 100 has title "T1" and is returned as id eYZmn. Set access_authorities so it is gated.
	_, err := app.writePool.Exec(ctx, `UPDATE tracks SET access_authorities = ARRAY[$1]::text[] WHERE track_id = 100 AND is_current = true`, gateWallet)
	require.NoError(t, err)

	// Without auth: track must not be returned
	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/full/tracks?id=eYZmn", &resp)
	assert.Equal(t, 200, status)
	assert.Len(t, resp.Data, 0, "tracks with access_authorities must not be returned when unauthenticated")

	// With auth signed by access authority: track must be returned
	status, _ = testGetWithWallet(t, app, "/v1/full/tracks?id=eYZmn", gateWallet, &resp)
	assert.Equal(t, 200, status)
	assert.Len(t, resp.Data, 1, "tracks with access_authorities must be returned when request is signed by matching authority")
	assert.Equal(t, "T1", resp.Data[0].Title.String)
	assert.Equal(t, []string{gateWallet}, resp.Data[0].AccessAuthorities)
}

func TestGetUnlistedTrackWithAccessAuthority(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	gateWallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	// Make track 100 both unlisted and gated by access_authorities
	_, err := app.writePool.Exec(ctx, `UPDATE tracks SET is_unlisted = true, access_authorities = ARRAY[$1]::text[] WHERE track_id = 100 AND is_current = true`, gateWallet)
	require.NoError(t, err)

	var resp struct {
		Data []dbv1.Track
	}

	// Without auth: unlisted + gated track must not be returned
	status, _ := testGet(t, app, "/v1/full/tracks?id=eYZmn", &resp)
	assert.Equal(t, 200, status)
	assert.Len(t, resp.Data, 0, "unlisted track with access_authorities must not be returned when unauthenticated")

	// With auth signed by access authority: unlisted track must be returned
	status, _ = testGetWithWallet(t, app, "/v1/full/tracks?id=eYZmn", gateWallet, &resp)
	assert.Equal(t, 200, status)
	assert.Len(t, resp.Data, 1, "unlisted track with access_authorities must be returned when request is signed by matching authority")
	assert.Equal(t, "T1", resp.Data[0].Title.String)
}
