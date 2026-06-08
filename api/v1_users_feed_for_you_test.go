package api

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// feedForYouItem mirrors the heterogenous feed envelope on the wire:
// {type, timestamp, item}. The item is left as raw JSON so tests can
// pick out whichever entity-shape fields they need.
type feedForYouItem struct {
	Type      string          `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Item      json.RawMessage `json:"item"`
}

// feedForYouFixtures builds a small graph that exercises the in-network,
// trending, and underground candidate sources for BOTH tracks and
// playlists, plus the standard filters.
//
//	user 1 = me (the viewer)
//	user 2 = an artist I follow         -> in-network track + playlist
//	user 3 = an artist I do NOT follow  -> trending track + playlist
//	user 4 = an underground artist      -> underground track + playlist
//	user 7 = a deactivated artist       -> filter test
//	user 8 = saved artist               -> already-saved filter
func feedForYouFixtures() database.FixtureMap {
	now := time.Now()
	hoursAgo := func(h int) time.Time { return now.Add(-time.Duration(h) * time.Hour) }
	users := []map[string]any{
		{"user_id": 1, "handle": "me", "handle_lc": "me", "wallet": "0x0000000000000000000000000000000000000001"},
		{"user_id": 2, "handle": "followed", "handle_lc": "followed", "wallet": "0x0000000000000000000000000000000000000002"},
		{"user_id": 3, "handle": "trending_artist", "handle_lc": "trending_artist", "wallet": "0x0000000000000000000000000000000000000003"},
		{"user_id": 4, "handle": "underground", "handle_lc": "underground", "wallet": "0x0000000000000000000000000000000000000004"},
		{"user_id": 7, "handle": "deactivated", "handle_lc": "deactivated", "wallet": "0x0000000000000000000000000000000000000007", "is_deactivated": true},
		{"user_id": 8, "handle": "saved_artist", "handle_lc": "saved_artist", "wallet": "0x0000000000000000000000000000000000000008"},
	}

	aggregateUser := []map[string]any{
		{"user_id": 1, "follower_count": 0, "following_count": 1},
		{"user_id": 2, "follower_count": 100, "following_count": 10},
		{"user_id": 3, "follower_count": 5000, "following_count": 200},
		{"user_id": 4, "follower_count": 100, "following_count": 50},
		{"user_id": 7, "follower_count": 0, "following_count": 0},
		{"user_id": 8, "follower_count": 200, "following_count": 50},
	}

	tracks := []map[string]any{
		{"track_id": 101, "owner_id": 2, "title": "in-network 1", "created_at": hoursAgo(2)},
		{"track_id": 102, "owner_id": 2, "title": "in-network 2", "created_at": hoursAgo(10)},
		{"track_id": 201, "owner_id": 3, "title": "trending", "created_at": hoursAgo(72)},
		{"track_id": 301, "owner_id": 4, "title": "underground", "created_at": hoursAgo(50)},
		{"track_id": 801, "owner_id": 8, "title": "already saved", "created_at": hoursAgo(100)},
		{"track_id": 701, "owner_id": 7, "title": "deactivated artist track", "created_at": hoursAgo(2)},
		{"track_id": 901, "owner_id": 2, "title": "unlisted", "created_at": hoursAgo(2), "is_unlisted": true},
		{"track_id": 902, "owner_id": 2, "title": "deleted", "created_at": hoursAgo(2), "is_delete": true},
	}

	playlists := []map[string]any{
		// In-network playlist: recent, by a user I follow.
		{"playlist_id": 1101, "playlist_owner_id": 2, "playlist_name": "in-network playlist", "is_album": false, "is_private": false, "playlist_contents": "{}", "created_at": hoursAgo(3), "updated_at": hoursAgo(3)},
		// Trending playlist by someone I don't follow.
		{"playlist_id": 1201, "playlist_owner_id": 3, "playlist_name": "trending playlist", "is_album": false, "is_private": false, "playlist_contents": "{}", "created_at": hoursAgo(96), "updated_at": hoursAgo(96)},
		// Underground playlist by sub-1500 follower artist.
		{"playlist_id": 1301, "playlist_owner_id": 4, "playlist_name": "underground playlist", "is_album": false, "is_private": false, "playlist_contents": "{}", "created_at": hoursAgo(60), "updated_at": hoursAgo(60)},
		// Already-saved playlist (should be filtered out).
		{"playlist_id": 1801, "playlist_owner_id": 8, "playlist_name": "already saved playlist", "is_album": false, "is_private": false, "playlist_contents": "{}", "created_at": hoursAgo(120), "updated_at": hoursAgo(120)},
		// Private playlist by a user I follow (should be filtered out).
		{"playlist_id": 1102, "playlist_owner_id": 2, "playlist_name": "private playlist", "is_album": false, "is_private": true, "playlist_contents": "{}", "created_at": hoursAgo(3), "updated_at": hoursAgo(3)},
		// Deleted playlist (should be filtered out).
		{"playlist_id": 1103, "playlist_owner_id": 2, "playlist_name": "deleted playlist", "is_album": false, "is_private": false, "playlist_contents": "{}", "created_at": hoursAgo(3), "updated_at": hoursAgo(3), "is_delete": true},
	}

	follows := []map[string]any{
		{"follower_user_id": 1, "followee_user_id": 2},
	}

	saves := []map[string]any{
		{"user_id": 1, "save_item_id": 801, "save_type": "track"},
		{"user_id": 1, "save_item_id": 1801, "save_type": "playlist"},
	}

	trackTrendingScores := []map[string]any{
		{"track_id": 201, "score": 9_000_000_000, "time_range": "week"},
		{"track_id": 301, "score": 5_000_000_000, "time_range": "week"},
	}

	playlistTrendingScores := []map[string]any{
		{"playlist_id": 1201, "type": "PLAYLISTS", "version": "pnagD", "time_range": "week", "score": 9_000_000_000.0},
		{"playlist_id": 1301, "type": "PLAYLISTS", "version": "pnagD", "time_range": "week", "score": 5_000_000_000.0},
	}

	aggregateTrack := []map[string]any{
		{"track_id": 101, "save_count": 5, "repost_count": 2},
		{"track_id": 102, "save_count": 1, "repost_count": 0},
		{"track_id": 201, "save_count": 100, "repost_count": 50},
		{"track_id": 301, "save_count": 30, "repost_count": 10},
	}

	aggregatePlaylist := []map[string]any{
		{"playlist_id": 1101, "is_album": false, "save_count": 4, "repost_count": 1},
		{"playlist_id": 1201, "is_album": false, "save_count": 80, "repost_count": 30},
		{"playlist_id": 1301, "is_album": false, "save_count": 20, "repost_count": 5},
	}

	return database.FixtureMap{
		"users":                    users,
		"aggregate_user":           aggregateUser,
		"tracks":                   tracks,
		"playlists":                playlists,
		"follows":                  follows,
		"saves":                    saves,
		"track_trending_scores":    trackTrendingScores,
		"playlist_trending_scores": playlistTrendingScores,
		"aggregate_track":          aggregateTrack,
		"aggregate_playlist":       aggregatePlaylist,
	}
}

// TestV1FeedForYou_RequiresValidUserId asserts the path :userId is
// validated by requireUserIdMiddleware — a junk hash id should be a 400,
// not silently treated as user 0.
func TestV1FeedForYou_RequiresValidUserId(t *testing.T) {
	app := emptyTestApp(t)
	status, _ := testGet(t, app, "/v1/users/not-a-real-id/feed/for-you")
	assert.Equal(t, 400, status)
}

// TestV1FeedForYou_EmptyFeedForNewUser asserts a brand-new user with no
// follows, no engagement, and no available candidates gets a 200 with an
// empty Data array — not a 500 or a crash. This is the cold-start case.
func TestV1FeedForYou_EmptyFeedForNewUser(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true

	// Only the viewer exists — no follows, no tracks, no trending rows,
	// no engagement. There's nothing for the pipeline to surface.
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 42, "handle": "newbie", "handle_lc": "newbie",
				"wallet": "0x0000000000000000000000000000000000000042"},
		},
	})

	var response struct {
		Data []feedForYouItem
	}
	path := "/v1/users/" + trashid.MustEncodeHashID(42) + "/feed/for-you"
	status, body := testGet(t, app, path, &response)
	require.Equal(t, 200, status, string(body))
	assert.Empty(t, response.Data, "new user with no graph should get an empty feed")
}

// TestV1FeedForYou_OldTrendingCandidateDoesNotUnderflow asserts that very old
// candidates decay to an effectively zero recency score instead of making
// PostgreSQL throw a numeric underflow from exp(...).
func TestV1FeedForYou_OldTrendingCandidateDoesNotUnderflow(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true

	fixtures := feedForYouFixtures()
	ancient := time.Date(1900, time.January, 1, 0, 0, 0, 0, time.UTC)
	fixtures["users"] = append(fixtures["users"], map[string]any{
		"user_id":   5,
		"handle":    "ancient_artist",
		"handle_lc": "ancient_artist",
		"wallet":    "0x0000000000000000000000000000000000000005",
	})
	fixtures["aggregate_user"] = append(fixtures["aggregate_user"], map[string]any{
		"user_id": 5, "follower_count": 5000, "following_count": 100,
	})
	fixtures["tracks"] = append(fixtures["tracks"], map[string]any{
		"track_id": 401, "owner_id": 5, "title": "ancient trending", "created_at": ancient,
	})
	fixtures["track_trending_scores"] = append(fixtures["track_trending_scores"], map[string]any{
		"track_id": 401, "score": 10_000_000_000, "time_range": "week",
	})
	database.Seed(app.pool.Replicas[0], fixtures)

	var resp struct {
		Data []feedForYouItem
	}
	path := fmt.Sprintf("/v1/users/%s/feed/for-you?limit=20", trashid.MustEncodeHashID(1))
	status, body := testGet(t, app, path, &resp)
	require.Equal(t, 200, status, string(body))
	assert.NotEmpty(t, resp.Data)
}

// TestV1FeedForYou_PaginationDoesNotRepeat asserts that two consecutive
// pages of the diversity-ordered list don't overlap on (type, id).
func TestV1FeedForYou_PaginationDoesNotRepeat(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true
	database.Seed(app.pool.Replicas[0], feedForYouFixtures())

	page := func(limit, offset int) []string {
		var resp struct {
			Data []feedForYouItem
		}
		path := fmt.Sprintf("/v1/users/%s/feed/for-you?limit=%d&offset=%d",
			trashid.MustEncodeHashID(1), limit, offset)
		status, body := testGet(t, app, path, &resp)
		require.Equal(t, 200, status, string(body))
		keys := make([]string, len(resp.Data))
		for i, it := range resp.Data {
			// Pull the id out of the wrapped track/playlist payload.
			var idHolder struct {
				ID         string `json:"id"`
				PlaylistID string `json:"playlist_id"`
				TrackID    string `json:"track_id"`
			}
			_ = json.Unmarshal(it.Item, &idHolder)
			id := idHolder.ID
			if id == "" {
				id = idHolder.PlaylistID
			}
			if id == "" {
				id = idHolder.TrackID
			}
			keys[i] = fmt.Sprintf("%s:%s", it.Type, id)
		}
		return keys
	}

	first := page(2, 0)
	second := page(2, 2)

	seen := map[string]bool{}
	for _, k := range first {
		seen[k] = true
	}
	for _, k := range second {
		assert.Falsef(t, seen[k], "item %s appeared on both pages", k)
	}
}

// TestV1FeedForYou_IncludesCollections asserts that with both tracks
// and playlists available across all three candidate sources, the feed
// surfaces a mix of types — not tracks-only.
func TestV1FeedForYou_IncludesCollections(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true
	database.Seed(app.pool.Replicas[0], feedForYouFixtures())

	var resp struct {
		Data []feedForYouItem
	}
	// Pull a wide page so all 3 in-network + 2 trending + 2 underground
	// playlists have a chance to land alongside the tracks.
	path := fmt.Sprintf("/v1/users/%s/feed/for-you?limit=20", trashid.MustEncodeHashID(1))
	status, body := testGet(t, app, path, &resp)
	require.Equal(t, 200, status, string(body))

	var (
		sawTrack    bool
		sawPlaylist bool
		ids         = map[string]bool{}
	)
	for _, it := range resp.Data {
		switch it.Type {
		case "track":
			sawTrack = true
		case "playlist":
			sawPlaylist = true
		}

		// Filtered/already-saved entities should never show up.
		var idHolder struct {
			PlaylistID string `json:"playlist_id"`
			TrackID    string `json:"track_id"`
		}
		_ = json.Unmarshal(it.Item, &idHolder)
		key := fmt.Sprintf("%s:%s%s", it.Type, idHolder.PlaylistID, idHolder.TrackID)
		ids[key] = true
	}
	assert.True(t, sawTrack, "expected at least one track in the for-you feed")
	assert.True(t, sawPlaylist, "expected at least one playlist in the for-you feed")
	assert.False(t, ids["playlist:"+trashid.MustEncodeHashID(1801)],
		"already-saved playlist 1801 must not appear")
	assert.False(t, ids["playlist:"+trashid.MustEncodeHashID(1102)],
		"private playlist 1102 must not appear")
	assert.False(t, ids["playlist:"+trashid.MustEncodeHashID(1103)],
		"deleted playlist 1103 must not appear")
	assert.False(t, ids["track:"+trashid.MustEncodeHashID(701)],
		"deactivated-owner track 701 must not appear")
}

// TestV1FeedForYou_PerArtistCapIsShared asserts that max_per_artist
// applies across both tracks and playlists for the same owner — the
// diversity cap is shared, not per-type. User 2 (followed) has 2 tracks
// and 1 in-network playlist; with max_per_artist=1 we should see at
// most 1 item from user 2 on a page.
func TestV1FeedForYou_PerArtistCapIsShared(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true
	database.Seed(app.pool.Replicas[0], feedForYouFixtures())

	var (
		_   dbv1.Track    // keep import alive even if Track shape is unused below
		_   dbv1.Playlist // ditto for Playlist
		_   = time.Time{} //
	)

	var resp struct {
		Data []feedForYouItem
	}
	path := fmt.Sprintf("/v1/users/%s/feed/for-you?limit=20&max_per_artist=1",
		trashid.MustEncodeHashID(1))
	status, body := testGet(t, app, path, &resp)
	require.Equal(t, 200, status, string(body))

	// Count entities authored by user 2. Track owner is `user_id`;
	// playlist owner is `user.user_id` inside the wrapped user object on
	// playlists. To stay schema-agnostic in this test we pull the owner
	// hash id out of either `user.id` (playlist shape) or `user.id`
	// (track shape) — both pin owner_id to the same nested user.
	type idHolder struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	user2 := trashid.MustEncodeHashID(2)
	var fromUser2 int
	for _, it := range resp.Data {
		var h idHolder
		_ = json.Unmarshal(it.Item, &h)
		if h.User.ID == user2 {
			fromUser2++
		}
	}
	assert.LessOrEqualf(t, fromUser2, 1,
		"max_per_artist=1 should produce at most 1 item from user 2 across track+playlist, got %d",
		fromUser2)
}
