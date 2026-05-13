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

// feedForYouFixtures builds a small graph that exercises every candidate
// source and every filter the For You feed cares about.
//
//	user 1  = me (the viewer)
//	user 2  = an artist I follow         -> in-network candidates
//	user 3  = an artist I do NOT follow  -> trending candidate
//	user 4  = an underground artist      -> underground candidate
//	user 5  = a "similar" artist         -> CF similar candidate (1-hop)
//	user 6  = a peer who shares a save   -> CF bridge user
//	user 7  = a deactivated artist       -> filter test
func feedForYouFixtures() database.FixtureMap {
	now := time.Now()
	hoursAgo := func(h int) time.Time { return now.Add(-time.Duration(h) * time.Hour) }
	users := []map[string]any{
		{"user_id": 1, "handle": "me", "handle_lc": "me", "wallet": "0x0000000000000000000000000000000000000001"},
		{"user_id": 2, "handle": "followed", "handle_lc": "followed", "wallet": "0x0000000000000000000000000000000000000002"},
		{"user_id": 3, "handle": "trending_artist", "handle_lc": "trending_artist", "wallet": "0x0000000000000000000000000000000000000003"},
		{"user_id": 4, "handle": "underground", "handle_lc": "underground", "wallet": "0x0000000000000000000000000000000000000004"},
		{"user_id": 5, "handle": "similar", "handle_lc": "similar", "wallet": "0x0000000000000000000000000000000000000005"},
		{"user_id": 6, "handle": "peer", "handle_lc": "peer", "wallet": "0x0000000000000000000000000000000000000006"},
		{"user_id": 7, "handle": "deactivated", "handle_lc": "deactivated", "wallet": "0x0000000000000000000000000000000000000007", "is_deactivated": true},
		{"user_id": 8, "handle": "saved_artist", "handle_lc": "saved_artist", "wallet": "0x0000000000000000000000000000000000000008"},
	}

	aggregateUser := []map[string]any{
		{"user_id": 1, "follower_count": 0, "following_count": 1},
		{"user_id": 2, "follower_count": 100, "following_count": 10},
		{"user_id": 3, "follower_count": 5000, "following_count": 200},
		// Underground artist: must be < 1500 on both counts.
		{"user_id": 4, "follower_count": 100, "following_count": 50},
		{"user_id": 5, "follower_count": 200, "following_count": 100},
		{"user_id": 6, "follower_count": 50, "following_count": 50},
		{"user_id": 7, "follower_count": 0, "following_count": 0},
		{"user_id": 8, "follower_count": 200, "following_count": 50},
	}

	tracks := []map[string]any{
		// In-network: two recent tracks by user 2 (the artist I follow).
		{"track_id": 101, "owner_id": 2, "title": "in-network 1", "created_at": hoursAgo(2)},
		{"track_id": 102, "owner_id": 2, "title": "in-network 2", "created_at": hoursAgo(10)},
		// Trending track by user 3 (in track_trending_scores).
		{"track_id": 201, "owner_id": 3, "title": "trending", "created_at": hoursAgo(72)},
		// Underground track by user 4 (also has trending score).
		{"track_id": 301, "owner_id": 4, "title": "underground", "created_at": hoursAgo(50)},
		// Similar-artist candidate: track by user 5.
		{"track_id": 501, "owner_id": 5, "title": "similar artist", "created_at": hoursAgo(30)},
		// Saved artist's track that I already saved -> must be filtered out.
		{"track_id": 801, "owner_id": 8, "title": "already saved", "created_at": hoursAgo(100)},
		// Track by deactivated user -> filtered.
		{"track_id": 701, "owner_id": 7, "title": "deactivated artist track", "created_at": hoursAgo(2)},
		// Track that is unlisted -> filtered.
		{"track_id": 901, "owner_id": 2, "title": "unlisted", "created_at": hoursAgo(2), "is_unlisted": true},
		// Track that is deleted -> filtered.
		{"track_id": 902, "owner_id": 2, "title": "deleted", "created_at": hoursAgo(2), "is_delete": true},
		// User 6's track (referenced by saves to drive CF; not a candidate
		// because user 6 is in my_saved_artists indirectly only via user 8).
	}

	follows := []map[string]any{
		// I follow user 2.
		{"follower_user_id": 1, "followee_user_id": 2},
	}

	saves := []map[string]any{
		// I have already saved track 801 (by user 8, my favorite artist).
		{"user_id": 1, "save_item_id": 801, "save_type": "track"},
		// User 6 saved 801 too (overlap with me on user 8).
		{"user_id": 6, "save_item_id": 801, "save_type": "track"},
		// User 6 also saved a track by user 5 -> drives "similar artist".
		{"user_id": 6, "save_item_id": 501, "save_type": "track"},
	}

	trackTrendingScores := []map[string]any{
		// Trending leader.
		{"track_id": 201, "score": 9_000_000_000, "time_range": "week"},
		// Underground (also trending but artist is sub-1500).
		{"track_id": 301, "score": 5_000_000_000, "time_range": "week"},
	}

	aggregateTrack := []map[string]any{
		{"track_id": 101, "save_count": 5, "repost_count": 2},
		{"track_id": 102, "save_count": 1, "repost_count": 0},
		{"track_id": 201, "save_count": 100, "repost_count": 50},
		{"track_id": 301, "save_count": 30, "repost_count": 10},
		{"track_id": 501, "save_count": 10, "repost_count": 5},
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

func TestV1FeedForYou_Basic(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true
	database.Seed(app.pool.Replicas[0], feedForYouFixtures())

	var response struct {
		Data []dbv1.Track
	}
	path := "/v1/users/" + trashid.MustEncodeHashID(1) + "/feed/for-you"
	status, body := testGet(t, app, path, &response)
	require.Equal(t, 200, status, string(body))

	// We should see candidates from at least the in-network and trending
	// sources, and we should NOT see any of the filtered tracks.
	gotIDs := map[int32]bool{}
	for _, tr := range response.Data {
		gotIDs[tr.TrackID] = true
	}

	// Filtered: already saved, deactivated, unlisted, deleted.
	for _, banned := range []int32{801, 701, 901, 902} {
		assert.Falsef(t, gotIDs[banned], "track %d should be filtered out", banned)
	}

	// At least one in-network track should appear.
	assert.True(t, gotIDs[101] || gotIDs[102], "expected an in-network track in results, got %v", gotIDs)

	// Trending track should appear (it's a wide-net candidate).
	assert.True(t, gotIDs[201], "expected trending track in results, got %v", gotIDs)
}

func TestV1FeedForYou_RequiresValidUserId(t *testing.T) {
	app := emptyTestApp(t)
	// Invalid hash id in the path → 400 from requireUserIdMiddleware.
	status, _ := testGet(t, app, "/v1/users/not-a-real-id/feed/for-you")
	assert.Equal(t, 400, status)
}

// /feed/for-you is exempt from authMiddleware's "if user_id is set, wallet
// must match" 403 — the query user_id is a viewer hint, not an authorization
// claim. This test pins that exemption: with skipAuthCheck OFF (so the real
// auth path runs) and no signature headers, the call still returns 200.
func TestV1FeedForYou_UnauthenticatedViewerIdAllowed(t *testing.T) {
	app := emptyTestApp(t)
	// Deliberately NOT setting app.skipAuthCheck — exercise the real
	// authMiddleware exemption added for this route.
	database.Seed(app.pool.Replicas[0], feedForYouFixtures())

	var response struct {
		Data []dbv1.Track
	}
	encodedId := trashid.MustEncodeHashID(1)
	path := "/v1/users/" + encodedId + "/feed/for-you?user_id=" + encodedId
	status, body := testGet(t, app, path, &response)
	require.Equal(t, 200, status, string(body))
}

func TestV1FeedForYou_ExcludesAlreadySavedTracks(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true
	database.Seed(app.pool.Replicas[0], feedForYouFixtures())

	var response struct {
		Data []dbv1.Track
	}
	path := "/v1/users/" + trashid.MustEncodeHashID(1) + "/feed/for-you"
	status, body := testGet(t, app, path, &response)
	require.Equal(t, 200, status, string(body))

	for _, tr := range response.Data {
		assert.NotEqual(t, int32(801), tr.TrackID, "already-saved track should be excluded")
	}
}

func TestV1FeedForYou_MaxThreePerArtist(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true

	// Build a user (the artist) who has many recent in-network tracks. The
	// feed should cap them at 3 per page.
	now := time.Now()
	hoursAgo := func(h int) time.Time { return now.Add(-time.Duration(h) * time.Hour) }
	users := []map[string]any{
		{"user_id": 1, "handle": "me", "handle_lc": "me", "wallet": "0x0000000000000000000000000000000000000001"},
		{"user_id": 2, "handle": "prolific", "handle_lc": "prolific", "wallet": "0x0000000000000000000000000000000000000002"},
	}
	tracks := make([]map[string]any, 10)
	for i := range tracks {
		tracks[i] = map[string]any{
			"track_id":   1000 + i,
			"owner_id":   2,
			"title":      fmt.Sprintf("prolific %d", i),
			"created_at": hoursAgo(i + 1),
		}
	}
	follows := []map[string]any{{"follower_user_id": 1, "followee_user_id": 2}}
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users":   users,
		"tracks":  tracks,
		"follows": follows,
	})

	var response struct {
		Data []dbv1.Track
	}
	path := "/v1/users/" + trashid.MustEncodeHashID(1) + "/feed/for-you?limit=20"
	status, body := testGet(t, app, path, &response)
	require.Equal(t, 200, status, string(body))

	count := 0
	for _, tr := range response.Data {
		if tr.UserID == trashid.HashId(2) {
			count++
		}
	}
	assert.LessOrEqual(t, count, 3, "expected at most 3 tracks per artist, got %d", count)
}

func TestV1FeedForYou_DiversityPassNoConsecutiveSameArtist(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true

	now := time.Now()
	hoursAgo := func(h int) time.Time { return now.Add(-time.Duration(h) * time.Hour) }

	// Two artists I follow, each with several recent tracks. The diversity
	// pass should interleave them so no two consecutive tracks share an
	// artist (when an alternative is available within the lookahead window).
	users := []map[string]any{
		{"user_id": 1, "handle": "me", "handle_lc": "me", "wallet": "0x0000000000000000000000000000000000000001"},
		{"user_id": 2, "handle": "a1", "handle_lc": "a1", "wallet": "0x0000000000000000000000000000000000000002"},
		{"user_id": 3, "handle": "a2", "handle_lc": "a2", "wallet": "0x0000000000000000000000000000000000000003"},
	}
	tracks := []map[string]any{
		{"track_id": 100, "owner_id": 2, "title": "a1-1", "created_at": hoursAgo(1)},
		{"track_id": 101, "owner_id": 2, "title": "a1-2", "created_at": hoursAgo(2)},
		{"track_id": 102, "owner_id": 2, "title": "a1-3", "created_at": hoursAgo(3)},
		{"track_id": 200, "owner_id": 3, "title": "a2-1", "created_at": hoursAgo(4)},
		{"track_id": 201, "owner_id": 3, "title": "a2-2", "created_at": hoursAgo(5)},
		{"track_id": 202, "owner_id": 3, "title": "a2-3", "created_at": hoursAgo(6)},
	}
	follows := []map[string]any{
		{"follower_user_id": 1, "followee_user_id": 2},
		{"follower_user_id": 1, "followee_user_id": 3},
	}
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users":   users,
		"tracks":  tracks,
		"follows": follows,
	})

	var response struct {
		Data []dbv1.Track
	}
	path := "/v1/users/" + trashid.MustEncodeHashID(1) + "/feed/for-you"
	status, body := testGet(t, app, path, &response)
	require.Equal(t, 200, status, string(body))
	require.GreaterOrEqual(t, len(response.Data), 4, "want at least 4 tracks to assert interleave")

	// Walk the response: for each pair, if there's any other artist in the
	// rest of the list, we should not have two same-artist tracks adjacent.
	for i := 1; i < len(response.Data); i++ {
		prev := response.Data[i-1].UserID
		cur := response.Data[i].UserID
		if prev == cur {
			// Only fail if a different-artist track existed elsewhere in the
			// page that could have been swapped in.
			otherExists := false
			for j := i; j < len(response.Data); j++ {
				if response.Data[j].UserID != prev {
					otherExists = true
					break
				}
			}
			if otherExists {
				t.Fatalf("adjacent same-artist tracks at pos %d/%d (artist %v); other artist available later in page", i-1, i, prev)
			}
		}
	}
}

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

func TestV1FeedForYou_InvalidParams(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true

	for _, val := range []string{"-1", "101", "invalid"} {
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/feed/for-you?limit="+val)
		assert.Equal(t, 400, status, "limit=%s", val)
	}
	for _, val := range []string{"-1", "invalid"} {
		status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/feed/for-you?offset="+val)
		assert.Equal(t, 400, status, "offset=%s", val)
	}
}

// TestV1FeedForYou_RecencyAndEngagementRanking isolates the ranking
// signals: all three artists are in-network (same source weight) and the
// caller has no prior engagement with any of them (uniform social_boost),
// so the only thing differentiating ranks is recency × engagement.
//
//	track A — fresh,  no engagement   → mid-rank
//	track B — old,    high engagement → low-rank (recency decay)
//	track C — fresh + high engagement → top-rank
func TestV1FeedForYou_RecencyAndEngagementRanking(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true

	const (
		me      = 200
		artistA = 201
		artistB = 202
		artistC = 203
	)
	now := time.Now()

	users := []map[string]any{
		{"user_id": me, "handle": "ranker_me", "handle_lc": "ranker_me",
			"wallet": "0x0000000000000000000000000000000000000200"},
		{"user_id": artistA, "handle": "artist_a", "handle_lc": "artist_a",
			"wallet": "0x0000000000000000000000000000000000000201"},
		{"user_id": artistB, "handle": "artist_b", "handle_lc": "artist_b",
			"wallet": "0x0000000000000000000000000000000000000202"},
		{"user_id": artistC, "handle": "artist_c", "handle_lc": "artist_c",
			"wallet": "0x0000000000000000000000000000000000000203"},
	}
	aggregateUser := []map[string]any{
		{"user_id": artistA, "follower_count": 100, "following_count": 50},
		{"user_id": artistB, "follower_count": 100, "following_count": 50},
		{"user_id": artistC, "follower_count": 100, "following_count": 50},
	}
	tracks := []map[string]any{
		{"track_id": 2001, "owner_id": artistA, "title": "fresh, low engagement",
			"created_at": now.Add(-1 * time.Hour)},
		{"track_id": 2002, "owner_id": artistB, "title": "old, high engagement",
			"created_at": now.Add(-13 * 24 * time.Hour)}, // ~13d, still inside the 14d in-network window
		{"track_id": 2003, "owner_id": artistC, "title": "fresh, high engagement",
			"created_at": now.Add(-2 * time.Hour)},
	}
	aggregateTrack := []map[string]any{
		{"track_id": 2001, "save_count": 0, "repost_count": 0},
		{"track_id": 2002, "save_count": 5_000, "repost_count": 1_000},
		{"track_id": 2003, "save_count": 5_000, "repost_count": 1_000},
	}
	follows := []map[string]any{
		{"follower_user_id": me, "followee_user_id": artistA},
		{"follower_user_id": me, "followee_user_id": artistB},
		{"follower_user_id": me, "followee_user_id": artistC},
	}
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users":           users,
		"aggregate_user":  aggregateUser,
		"tracks":          tracks,
		"aggregate_track": aggregateTrack,
		"follows":         follows,
	})

	var response struct {
		Data []dbv1.Track
	}
	path := "/v1/users/" + trashid.MustEncodeHashID(me) + "/feed/for-you"
	status, body := testGet(t, app, path, &response)
	require.Equal(t, 200, status, string(body))
	require.GreaterOrEqual(t, len(response.Data), 3, "expected all three candidates in response")

	idx := func(id int32) int {
		for i, tr := range response.Data {
			if tr.TrackID == id {
				return i
			}
		}
		return -1
	}
	assert.Less(t, idx(2003), idx(2001),
		"fresh+engaged 2003 must outrank low-engagement 2001")
	assert.Less(t, idx(2003), idx(2002),
		"fresh+engaged 2003 must outrank old 2002 (recency decay)")
}
