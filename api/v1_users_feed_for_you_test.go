package api

import (
	"fmt"
	"testing"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// feedForYouFixtures builds a small graph that exercises the in-network
// and trending candidate sources plus the standard filters.
//
//	user 1 = me (the viewer)
//	user 2 = an artist I follow         -> in-network candidates
//	user 3 = an artist I do NOT follow  -> trending candidate
//	user 4 = an underground artist      -> underground candidate
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

	follows := []map[string]any{
		{"follower_user_id": 1, "followee_user_id": 2},
	}

	saves := []map[string]any{
		{"user_id": 1, "save_item_id": 801, "save_type": "track"},
	}

	trackTrendingScores := []map[string]any{
		{"track_id": 201, "score": 9_000_000_000, "time_range": "week"},
		{"track_id": 301, "score": 5_000_000_000, "time_range": "week"},
	}

	aggregateTrack := []map[string]any{
		{"track_id": 101, "save_count": 5, "repost_count": 2},
		{"track_id": 102, "save_count": 1, "repost_count": 0},
		{"track_id": 201, "save_count": 100, "repost_count": 50},
		{"track_id": 301, "save_count": 30, "repost_count": 10},
	}

	return database.FixtureMap{
		"users":                 users,
		"aggregate_user":        aggregateUser,
		"tracks":                tracks,
		"follows":               follows,
		"saves":                 saves,
		"track_trending_scores": trackTrendingScores,
		"aggregate_track":       aggregateTrack,
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
		Data []dbv1.Track
	}
	path := "/v1/users/" + trashid.MustEncodeHashID(42) + "/feed/for-you"
	status, body := testGet(t, app, path, &response)
	require.Equal(t, 200, status, string(body))
	assert.Empty(t, response.Data, "new user with no graph should get an empty feed")
}

// TestV1FeedForYou_PaginationDoesNotRepeat asserts that two consecutive
// pages of the diversity-ordered list don't overlap on track ids.
func TestV1FeedForYou_PaginationDoesNotRepeat(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true
	database.Seed(app.pool.Replicas[0], feedForYouFixtures())

	page := func(limit, offset int) []int32 {
		var resp struct {
			Data []dbv1.Track
		}
		path := fmt.Sprintf("/v1/users/%s/feed/for-you?limit=%d&offset=%d",
			trashid.MustEncodeHashID(1), limit, offset)
		status, body := testGet(t, app, path, &resp)
		require.Equal(t, 200, status, string(body))
		ids := make([]int32, len(resp.Data))
		for i, tr := range resp.Data {
			ids[i] = tr.TrackID
		}
		return ids
	}

	first := page(2, 0)
	second := page(2, 2)

	seen := map[int32]bool{}
	for _, id := range first {
		seen[id] = true
	}
	for _, id := range second {
		assert.Falsef(t, seen[id], "track %d appeared on both pages", id)
	}
}
