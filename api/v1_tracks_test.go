package api

import (
	"context"
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
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

func TestGetTracksByISRC(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	// Track 100 ("T1") stored with dashes; track 101 ("T2") stored without.
	_, err := app.writePool.Exec(ctx,
		`UPDATE tracks SET isrc = 'US-ANG-21-03742' WHERE track_id = 100 AND is_current = true`)
	require.NoError(t, err)
	_, err = app.writePool.Exec(ctx,
		`UPDATE tracks SET isrc = 'QMEU31610080' WHERE track_id = 101 AND is_current = true`)
	require.NoError(t, err)

	track100Id := trashid.MustEncodeHashID(100)
	track101Id := trashid.MustEncodeHashID(101)

	cases := []struct {
		name   string
		query  string
		wantId string
	}{
		{"stored-with-dashes, queried without", "USANG2103742", track100Id},
		{"stored-with-dashes, queried with same dashes", "US-ANG-21-03742", track100Id},
		{"stored-with-dashes, queried lowercased and undashed", "usang2103742", track100Id},
		{"stored-without-dashes, queried as-is", "QMEU31610080", track101Id},
		{"stored-without-dashes, queried with inserted dashes", "QM-EU3-16-10080", track101Id},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var resp struct {
				Data []dbv1.Track
			}
			status, body := testGet(t, app, "/v1/tracks?isrc="+tc.query, &resp)
			require.Equal(t, 200, status, string(body))
			require.Len(t, resp.Data, 1, "expected exactly one track for %q: %s", tc.query, string(body))
			jsonAssert(t, body, map[string]any{"data.0.id": tc.wantId})
		})
	}
}

func TestGetUnlistedTrackByPermalinkAnonymous(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	// Mark the permalink fixture track (track_id=500) as unlisted.
	_, err := app.writePool.Exec(ctx, `UPDATE tracks SET is_unlisted = true WHERE track_id = 500 AND is_current = true`)
	require.NoError(t, err)

	// Anonymous request via permalink returns the unlisted track.
	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/full/tracks?permalink=/TracksByPermalink/track-by-permalink", &resp)
	assert.Equal(t, 200, status)
	assert.Len(t, resp.Data, 1, "permalink lookup must return the unlisted track even without auth")
	assert.Equal(t, "track by permalink", resp.Data[0].Title.String)
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
