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

// usersFeedForYouFixtures builds a graph that exercises every candidate
// source for the user-scoped For You feed:
//
//	user 1  = me (the viewer)
//	user 2  = an artist I follow                  -> following candidates
//	user 3  = a popular trending artist           -> trending candidate
//	user 4  = an underground artist (sub-1500 fols) -> underground candidate
//	user 5  = a recommended-genre artist          -> recommended candidate
//	user 6  = a deactivated artist                -> filter test
//	user 7  = an artist whose track I've saved    -> dedupe-as-favorite test
func usersFeedForYouFixtures() database.FixtureMap {
	now := time.Now()
	hoursAgo := func(h int) time.Time { return now.Add(-time.Duration(h) * time.Hour) }

	users := []map[string]any{
		{"user_id": 1, "handle": "me", "handle_lc": "me", "wallet": "0x0000000000000000000000000000000000000001"},
		{"user_id": 2, "handle": "followed", "handle_lc": "followed", "wallet": "0x0000000000000000000000000000000000000002"},
		{"user_id": 3, "handle": "trending_artist", "handle_lc": "trending_artist", "wallet": "0x0000000000000000000000000000000000000003"},
		{"user_id": 4, "handle": "underground", "handle_lc": "underground", "wallet": "0x0000000000000000000000000000000000000004"},
		{"user_id": 5, "handle": "rec_artist", "handle_lc": "rec_artist", "wallet": "0x0000000000000000000000000000000000000005"},
		{"user_id": 6, "handle": "deactivated", "handle_lc": "deactivated", "wallet": "0x0000000000000000000000000000000000000006", "is_deactivated": true},
		{"user_id": 7, "handle": "saved_artist", "handle_lc": "saved_artist", "wallet": "0x0000000000000000000000000000000000000007"},
	}

	aggregateUser := []map[string]any{
		{"user_id": 1, "follower_count": 0, "following_count": 1},
		{"user_id": 2, "follower_count": 100, "following_count": 10},
		{"user_id": 3, "follower_count": 5000, "following_count": 200}, // not underground
		{"user_id": 4, "follower_count": 100, "following_count": 50},   // underground
		{"user_id": 5, "follower_count": 200, "following_count": 100},
		{"user_id": 6, "follower_count": 0, "following_count": 0},
		{"user_id": 7, "follower_count": 100, "following_count": 50},
	}

	tracks := []map[string]any{
		// Following candidates: recent uploads by user 2.
		{"track_id": 101, "owner_id": 2, "title": "following 1", "created_at": hoursAgo(2), "genre": "Rock"},
		{"track_id": 102, "owner_id": 2, "title": "following 2", "created_at": hoursAgo(10), "genre": "Rock"},
		// Trending candidate by user 3 (>=1500 followers).
		{"track_id": 201, "owner_id": 3, "title": "trending", "created_at": hoursAgo(48), "genre": "Pop"},
		// Underground candidate by user 4 (sub-1500).
		{"track_id": 301, "owner_id": 4, "title": "underground", "created_at": hoursAgo(50), "genre": "Pop"},
		// Recommended candidate (top-genre Rock by user 5).
		{"track_id": 501, "owner_id": 5, "title": "rock recommendation", "created_at": hoursAgo(30), "genre": "Rock"},
		// Already-saved track (favorite filter): user 7.
		{"track_id": 701, "owner_id": 7, "title": "already saved", "created_at": hoursAgo(80), "genre": "Pop"},
		// Track by deactivated user (must be filtered).
		{"track_id": 601, "owner_id": 6, "title": "deactivated", "created_at": hoursAgo(2), "genre": "Pop"},
		// Played track by user 1 (drives top_genres = Rock; played -> excluded
		// from recommended).
		{"track_id": 401, "owner_id": 5, "title": "i played this", "created_at": hoursAgo(100), "genre": "Rock"},
		// Unlisted by user 2 (must be filtered).
		{"track_id": 901, "owner_id": 2, "title": "unlisted", "created_at": hoursAgo(2), "is_unlisted": true},
		// Deleted by user 2 (must be filtered).
		{"track_id": 902, "owner_id": 2, "title": "deleted", "created_at": hoursAgo(2), "is_delete": true},
	}

	follows := []map[string]any{
		// I follow user 2.
		{"follower_user_id": 1, "followee_user_id": 2},
	}

	// I have one play (track 401, Rock) -> top_genres = Rock.
	plays := []map[string]any{
		{"id": 1, "user_id": 1, "play_item_id": 401, "created_at": hoursAgo(100)},
	}

	// I have already saved track 701 (favorite filter input).
	saves := []map[string]any{
		{"user_id": 1, "save_item_id": 701, "save_type": "track"},
	}

	trackTrendingScores := []map[string]any{
		// Trending leaders, ungenred (matches trending/underground sources).
		{"track_id": 201, "score": 9_000_000_000, "time_range": "week", "genre": ""},
		{"track_id": 301, "score": 5_000_000_000, "time_range": "week", "genre": ""},
		{"track_id": 701, "score": 4_000_000_000, "time_range": "week", "genre": ""},
		// Genre-tagged Rock scores drive `recommended` (top-genre source).
		{"track_id": 501, "score": 8_000_000_000, "time_range": "week", "genre": "Rock"},
		{"track_id": 401, "score": 7_500_000_000, "time_range": "week", "genre": "Rock"},
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
		"plays":                 plays,
		"saves":                 saves,
		"track_trending_scores": trackTrendingScores,
		"aggregate_track":       aggregateTrack,
	}
}

func TestV1UsersFeedForYou_Basic(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true
	database.Seed(app.pool.Replicas[0], usersFeedForYouFixtures())

	var response struct {
		Data []dbv1.Track
	}
	path := "/v1/users/" + trashid.MustEncodeHashID(1) + "/feed/for-you?limit=20"
	status, body := testGet(t, app, path, &response)
	require.Equal(t, 200, status, string(body))

	gotIDs := map[int32]bool{}
	for _, tr := range response.Data {
		gotIDs[tr.TrackID] = true
	}

	// Filtered tracks must not appear: deactivated owner, unlisted,
	// deleted, already-saved.
	for _, banned := range []int32{601, 901, 902, 701} {
		assert.Falsef(t, gotIDs[banned], "track %d should be filtered out, got %v", banned, gotIDs)
	}

	// At least one following track should appear.
	assert.True(t, gotIDs[101] || gotIDs[102], "expected a following track in results, got %v", gotIDs)

	// The trending and underground tracks should both appear.
	assert.True(t, gotIDs[201], "expected trending track 201 in results, got %v", gotIDs)
	assert.True(t, gotIDs[301], "expected underground track 301 in results, got %v", gotIDs)

	// Recommended candidate (501) should appear (top-genre Rock,
	// excluded play is 401).
	assert.True(t, gotIDs[501], "expected recommended track 501 in results, got %v", gotIDs)
	// Played track (401) should NOT appear in recommended.
	assert.False(t, gotIDs[401], "played track 401 should be excluded from recommended")
}

func TestV1UsersFeedForYou_RequiresValidUserId(t *testing.T) {
	app := emptyTestApp(t)
	status, _ := testGet(t, app, "/v1/users/not-a-real-id/feed/for-you")
	assert.Equal(t, 400, status)
}

func TestV1UsersFeedForYou_ExcludesAlreadySavedTracks(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true
	database.Seed(app.pool.Replicas[0], usersFeedForYouFixtures())

	var response struct {
		Data []dbv1.Track
	}
	path := "/v1/users/" + trashid.MustEncodeHashID(1) + "/feed/for-you?limit=50"
	status, body := testGet(t, app, path, &response)
	require.Equal(t, 200, status, string(body))

	for _, tr := range response.Data {
		assert.NotEqual(t, int32(701), tr.TrackID, "already-saved track 701 should be filtered")
	}
}

func TestV1UsersFeedForYou_DedupeAcrossSources(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true

	// Track 100 is reachable from BOTH the following source (user 2 is
	// followed) AND the trending source (it has an ungenred trending
	// score). It must appear exactly once in the response.
	now := time.Now()
	users := []map[string]any{
		{"user_id": 1, "handle": "me", "handle_lc": "me", "wallet": "0x0000000000000000000000000000000000000010"},
		{"user_id": 2, "handle": "dual", "handle_lc": "dual", "wallet": "0x0000000000000000000000000000000000000020"},
	}
	aggregateUser := []map[string]any{
		{"user_id": 2, "follower_count": 5000, "following_count": 100},
	}
	tracks := []map[string]any{
		{"track_id": 100, "owner_id": 2, "title": "dual-source", "created_at": now.Add(-2 * time.Hour)},
	}
	follows := []map[string]any{
		{"follower_user_id": 1, "followee_user_id": 2},
	}
	trackTrendingScores := []map[string]any{
		{"track_id": 100, "score": 9_000_000_000, "time_range": "week", "genre": ""},
	}
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users":                 users,
		"aggregate_user":        aggregateUser,
		"tracks":                tracks,
		"follows":               follows,
		"track_trending_scores": trackTrendingScores,
	})

	var response struct {
		Data []dbv1.Track
	}
	path := "/v1/users/" + trashid.MustEncodeHashID(1) + "/feed/for-you?limit=20"
	status, body := testGet(t, app, path, &response)
	require.Equal(t, 200, status, string(body))

	count := 0
	for _, tr := range response.Data {
		if tr.TrackID == 100 {
			count++
		}
	}
	assert.Equal(t, 1, count, "track 100 should appear exactly once after dedupe; saw %d", count)
}

func TestV1UsersFeedForYou_PaginationDoesNotRepeat(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true
	database.Seed(app.pool.Replicas[0], usersFeedForYouFixtures())

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

func TestV1UsersFeedForYou_InvalidParams(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true

	for _, val := range []string{"-1", "101", "invalid"} {
		path := "/v1/users/" + trashid.MustEncodeHashID(1) + "/feed/for-you?limit=" + val
		status, _ := testGet(t, app, path)
		assert.Equal(t, 400, status, "limit=%s", val)
	}
	for _, val := range []string{"-1", "201", "invalid"} {
		path := "/v1/users/" + trashid.MustEncodeHashID(1) + "/feed/for-you?offset=" + val
		status, _ := testGet(t, app, path)
		assert.Equal(t, 400, status, "offset=%s", val)
	}
}

// TestV1UsersFeedForYou_InterleaveBlend exercises the slot pattern. Seed
// each source with an unambiguous, named track, then assert that all four
// sources contributed at least one track within a single 10-slot page.
//
//	track 1100, 1101  -> following (user 2)
//	track 1200        -> trending  (user 3, no genre on trending score)
//	track 1300        -> underground (user 4, sub-1500 follower & following)
//	track 1500        -> recommended (top-genre Rock; user 5)
//	track 1600 (Rock) -> a played track that drives top_genres = Rock
func TestV1UsersFeedForYou_InterleaveBlend(t *testing.T) {
	app := emptyTestApp(t)
	app.skipAuthCheck = true

	now := time.Now()
	hoursAgo := func(h int) time.Time { return now.Add(-time.Duration(h) * time.Hour) }

	users := []map[string]any{
		{"user_id": 1, "handle": "blender_me", "handle_lc": "blender_me",
			"wallet": "0x0000000000000000000000000000000000000100"},
		{"user_id": 2, "handle": "blender_following", "handle_lc": "blender_following",
			"wallet": "0x0000000000000000000000000000000000000200"},
		{"user_id": 3, "handle": "blender_trending", "handle_lc": "blender_trending",
			"wallet": "0x0000000000000000000000000000000000000300"},
		{"user_id": 4, "handle": "blender_underground", "handle_lc": "blender_underground",
			"wallet": "0x0000000000000000000000000000000000000400"},
		{"user_id": 5, "handle": "blender_rec", "handle_lc": "blender_rec",
			"wallet": "0x0000000000000000000000000000000000000500"},
	}
	aggregateUser := []map[string]any{
		{"user_id": 2, "follower_count": 100, "following_count": 100},
		{"user_id": 3, "follower_count": 5000, "following_count": 200},
		{"user_id": 4, "follower_count": 100, "following_count": 50},
		{"user_id": 5, "follower_count": 100, "following_count": 100},
	}
	tracks := []map[string]any{
		{"track_id": 1100, "owner_id": 2, "title": "follow1", "created_at": hoursAgo(1)},
		{"track_id": 1101, "owner_id": 2, "title": "follow2", "created_at": hoursAgo(2)},
		{"track_id": 1200, "owner_id": 3, "title": "trending1", "created_at": hoursAgo(20)},
		{"track_id": 1300, "owner_id": 4, "title": "underground1", "created_at": hoursAgo(20)},
		{"track_id": 1500, "owner_id": 5, "title": "rec1", "created_at": hoursAgo(20), "genre": "Rock"},
		// played track to seed top_genres = Rock
		{"track_id": 1600, "owner_id": 5, "title": "ihaveplayed", "created_at": hoursAgo(50), "genre": "Rock"},
	}
	follows := []map[string]any{
		{"follower_user_id": 1, "followee_user_id": 2},
	}
	plays := []map[string]any{
		{"id": 9001, "user_id": 1, "play_item_id": 1600, "created_at": hoursAgo(50)},
	}
	trackTrendingScores := []map[string]any{
		// Trending and underground are ungenred.
		{"track_id": 1200, "score": 9_000_000_000, "time_range": "week", "genre": ""},
		{"track_id": 1300, "score": 5_000_000_000, "time_range": "week", "genre": ""},
		// Recommended is genre-tagged Rock.
		{"track_id": 1500, "score": 8_000_000_000, "time_range": "week", "genre": "Rock"},
		{"track_id": 1600, "score": 7_000_000_000, "time_range": "week", "genre": "Rock"},
	}
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users":                 users,
		"aggregate_user":        aggregateUser,
		"tracks":                tracks,
		"follows":               follows,
		"plays":                 plays,
		"track_trending_scores": trackTrendingScores,
	})

	var response struct {
		Data []dbv1.Track
	}
	path := "/v1/users/" + trashid.MustEncodeHashID(1) + "/feed/for-you?limit=10"
	status, body := testGet(t, app, path, &response)
	require.Equal(t, 200, status, string(body))

	got := map[int32]bool{}
	for _, tr := range response.Data {
		got[tr.TrackID] = true
	}

	// All four sources should have contributed at least one track to
	// the first 10-slot page.
	assert.True(t, got[1100] || got[1101], "expected a following track in the first page, got %v", got)
	assert.True(t, got[1200], "expected the trending track in the first page, got %v", got)
	assert.True(t, got[1300], "expected the underground track in the first page, got %v", got)
	assert.True(t, got[1500], "expected the recommended track in the first page, got %v", got)
}

// TestBlendForYouSources_Pattern unit-tests the interleave with synthetic
// ranked lists and asserts the canonical 10-slot ordering when every
// source is fully populated.
func TestBlendForYouSources_Pattern(t *testing.T) {
	bySrc := map[string][]int32{
		"recommended": {10, 11, 12, 13, 14, 15, 16, 17, 18, 19},
		"following":   {20, 21, 22, 23, 24, 25, 26, 27, 28, 29},
		"trending":    {30, 31, 32, 33, 34, 35, 36, 37, 38, 39},
		"underground": {40, 41, 42, 43, 44, 45, 46, 47, 48, 49},
	}
	got := blendForYouSources(bySrc, 10)
	want := []int32{10, 11, 12, 20, 13, 30, 14, 21, 40, 15}
	assert.Equal(t, want, got, "first 10 slots should follow [R R R F R T R F U R]")
}

// TestBlendForYouSources_FallthroughWhenSourceEmpty verifies that an
// empty source delegates to the next source in the fallback order.
func TestBlendForYouSources_FallthroughWhenSourceEmpty(t *testing.T) {
	// No `following` candidates at all; following slots should be
	// filled by the next priority source (recommended).
	bySrc := map[string][]int32{
		"recommended": {10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20},
		"following":   {},
		"trending":    {30},
		"underground": {40},
	}
	got := blendForYouSources(bySrc, 10)
	// At slot 3 (following), recommended fills in (13). At slot 7
	// (following again), recommended fills in (15).
	want := []int32{10, 11, 12, 13, 14, 30, 15, 16, 40, 17}
	assert.Equal(t, want, got)
}

// TestBlendForYouSources_GlobalDedupe verifies that an id appearing in
// multiple sources is emitted only once.
func TestBlendForYouSources_GlobalDedupe(t *testing.T) {
	bySrc := map[string][]int32{
		"recommended": {1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		"following":   {2, 11, 12}, // 2 collides with recommended
		"trending":    {3, 13},     // 3 collides
		"underground": {1, 14},     // 1 collides
	}
	got := blendForYouSources(bySrc, 10)
	seen := map[int32]int{}
	for _, id := range got {
		seen[id]++
	}
	for id, n := range seen {
		assert.Equalf(t, 1, n, "track id %d emitted %d times", id, n)
	}
}
