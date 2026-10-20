package api

import (
	"fmt"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestFeedScheduledReleases(t *testing.T) {
	app := emptyTestApp(t)
	const wallet = "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	now := time.Now().UTC()
	fixtures := feedForYouFixtures()
	fixtures["users"][1]["wallet"] = wallet // owner 2, verified with a real test signature
	fixtures["tracks"] = nil
	fixtures["playlists"] = nil
	fixtures["reposts"] = nil
	fixtures["track_trending_scores"] = nil
	fixtures["playlist_trending_scores"] = nil
	// Include public-but-scheduled rows: visibility flags alone must not leak them.
	cases := []struct {
		name              string
		scheduled, hidden bool
		release           any
		visible           bool
	}{
		{"scheduled public future", true, false, now.Add(time.Hour), false},
		{"scheduled hidden future", true, true, now.Add(time.Hour), false},
		{"released", true, false, now.Add(-time.Hour), true},
		{"public future date", false, false, now.Add(time.Hour), true},
		{"ordinary hidden", false, true, nil, false},
		{"scheduled missing date", true, false, nil, false},
	}
	for i, tc := range cases {
		id := 500 + i
		fixtures["tracks"] = append(fixtures["tracks"], map[string]any{
			"track_id": id, "owner_id": 2, "title": tc.name, "created_at": now.Add(-time.Hour),
			"is_scheduled_release": tc.scheduled, "is_unlisted": tc.hidden, "release_date": tc.release,
		})
		fixtures["playlists"] = append(fixtures["playlists"], map[string]any{
			"playlist_id": id, "playlist_owner_id": 2, "playlist_name": tc.name, "is_album": true,
			"playlist_contents": "{}", "created_at": now.Add(-time.Hour), "updated_at": now,
			"is_scheduled_release": tc.scheduled, "is_private": tc.hidden, "release_date": tc.release,
		})
		for _, kind := range []string{"track", "album"} {
			fixtures["reposts"] = append(fixtures["reposts"], map[string]any{
				"user_id": 2, "repost_type": kind, "repost_item_id": id, "created_at": now.Add(-time.Minute),
			})
		}
		fixtures["track_trending_scores"] = append(fixtures["track_trending_scores"], map[string]any{"track_id": id, "score": 1000 - i, "time_range": "week"})
		fixtures["playlist_trending_scores"] = append(fixtures["playlist_trending_scores"], map[string]any{"playlist_id": id, "score": 1000 - i, "type": "PLAYLISTS", "version": "pnagD", "time_range": "week"})
	}
	database.Seed(app.pool.Replicas[0], fixtures)
	for _, route := range []string{"feed?filter=original", "feed?filter=repost", "feed/for-you?max_per_artist=10"} {
		path := "/v1/users/" + trashid.MustEncodeHashID(1) + "/" + route + "&limit=100"
		t.Run(route, func(t *testing.T) {
			for _, owner := range []bool{false, true} {
				url, signer := path, ""
				if owner {
					url += "&user_id=" + trashid.MustEncodeHashID(2)
					signer = wallet
				}
				status, body := testGetWithWallet(t, app, url, signer)
				require.Equal(t, 200, status, string(body))
				for i, tc := range cases {
					expected := tc.visible || (owner && tc.scheduled)
					for _, field := range []string{"title", "playlist_name"} {
						found := false
						for _, item := range gjson.GetBytes(body, "data").Array() {
							if item.Get("item."+field).String() == tc.name {
								found = true
							}
						}
						assert.Equal(t, expected, found, "%s case %d %s owner=%v: %s", field, i, tc.name, owner, body)
					}
				}
			}
		})
	}
	// For You permits advisory viewer IDs; unsigned ownership must not unlock releases.
	path := fmt.Sprintf("/v1/users/%s/feed/for-you?user_id=%s&max_per_artist=10", trashid.MustEncodeHashID(1), trashid.MustEncodeHashID(2))
	status, body := testGet(t, app, path)
	require.Equal(t, 200, status, string(body))
	for _, item := range gjson.GetBytes(body, "data").Array() {
		assert.NotEqual(t, "scheduled public future", item.Get("item.title").String())
		assert.NotEqual(t, "scheduled public future", item.Get("item.playlist_name").String())
		assert.NotEqual(t, "scheduled hidden future", item.Get("item.title").String())
		assert.NotEqual(t, "scheduled hidden future", item.Get("item.playlist_name").String())
	}
}
