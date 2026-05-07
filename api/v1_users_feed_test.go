package api

import (
	"context"
	"testing"

	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

// Regression coverage for the feed query. The query was rewritten to split
// repost handling into separate track-type and playlist-type branches so the
// planner stops building a hash over every public playlist on every call —
// this test guards both branches plus the owned-track and owned-playlist
// branches.
func TestUsersFeed(t *testing.T) {
	app := testAppWithFixtures(t)
	app.skipAuthCheck = true

	// Seed a playlist repost so Branch 1b (playlist/album reposts) executes
	// alongside the existing track repost in RepostFixtures (user 1 → track
	// 200). Reposts pkey is (user_id, repost_item_id, repost_type, txhash).
	_, err := app.pool.Exec(context.Background(), `
		INSERT INTO reposts (user_id, repost_type, repost_item_id, txhash, blockhash, blocknumber, created_at, is_delete, is_current)
		VALUES (3, 'playlist', 1, 'feed-test-tx-1', 'block1', 101, now() - interval '1 hour', false, true)
	`)
	assert.NoError(t, err)

	// User 2 follows user 1 (track repost path) and user 3 (playlist repost
	// path) per fixtures. Feed should surface both.
	var resp struct {
		Data []struct {
			Type string `json:"type"`
		}
	}
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(2)+"/feed?limit=50", &resp)
	assert.Equal(t, 200, status)
	assert.NotEmpty(t, resp.Data, "feed for user 2 (2 followees) should not be empty")

	// Spot-check the items the data contains.
	titles := []string{}
	for _, m := range gjson.GetBytes(body, "data.#.item.title").Array() {
		titles = append(titles, m.String())
	}
	playlistNames := []string{}
	for _, m := range gjson.GetBytes(body, "data.#.item.playlist_name").Array() {
		playlistNames = append(playlistNames, m.String())
	}

	// Track 200 (Culca Canyon) is owned by user 2 themselves but reposted by
	// user 1; it should appear via the track-repost branch.
	assert.Contains(t, titles, "Culca Canyon", "feed should include track reposted by a followee")

	// Playlist 1 (First) is owned by user 1 and now reposted by user 3; it
	// should appear via the playlist-repost branch.
	assert.Contains(t, playlistNames, "First", "feed should include playlist reposted by a followee")

	// Sanity: a user with zero followees gets an empty feed.
	var empty struct {
		Data []any
	}
	status, _ = testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(99999)+"/feed?limit=10", &empty)
	assert.Equal(t, 200, status)
	assert.Empty(t, empty.Data)
}
