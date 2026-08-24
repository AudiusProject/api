package api

import (
	"context"
	"testing"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// discoverWeeklyFixtures builds a graph covering every filter and both
// candidate sources.
//
//	user 1  = me (the viewer). Plays rock, so rock is my affinity genre.
//	user 2  = big unfollowed artist       -> the expected top pick
//	user 3  = underground artist          -> underground source
//	user 4  = an artist I follow          -> demoted, not excluded
//	user 5  = deactivated artist          -> filtered
//	user 6  = artist whose track I played -> filtered
//	user 7  = artist whose track I saved  -> filtered
//	user 8  = artist with a gated track   -> filtered
//	user 9  = artist with an ancient track-> filtered (age)
//	user 10 = artist with two good tracks -> one-per-artist cap
func discoverWeeklyFixtures() database.FixtureMap {
	now := time.Now()
	daysAgo := func(d int) time.Time { return now.AddDate(0, 0, -d) }

	users := []map[string]any{
		{"user_id": 1, "handle": "me", "handle_lc": "me", "wallet": "0x0000000000000000000000000000000000000001"},
		{"user_id": 2, "handle": "bigartist", "handle_lc": "bigartist", "wallet": "0x0000000000000000000000000000000000000002"},
		{"user_id": 3, "handle": "underground", "handle_lc": "underground", "wallet": "0x0000000000000000000000000000000000000003"},
		{"user_id": 4, "handle": "followed", "handle_lc": "followed", "wallet": "0x0000000000000000000000000000000000000004"},
		{"user_id": 5, "handle": "deactivated", "handle_lc": "deactivated", "is_deactivated": true, "wallet": "0x0000000000000000000000000000000000000005"},
		{"user_id": 6, "handle": "alreadyplayed", "handle_lc": "alreadyplayed", "wallet": "0x0000000000000000000000000000000000000006"},
		{"user_id": 7, "handle": "alreadysaved", "handle_lc": "alreadysaved", "wallet": "0x0000000000000000000000000000000000000007"},
		{"user_id": 8, "handle": "gated", "handle_lc": "gated", "wallet": "0x0000000000000000000000000000000000000008"},
		{"user_id": 9, "handle": "ancient", "handle_lc": "ancient", "wallet": "0x0000000000000000000000000000000000000009"},
		{"user_id": 10, "handle": "twotracks", "handle_lc": "twotracks", "wallet": "0x000000000000000000000000000000000000000a"},
	}

	// Everyone is over the underground threshold except user 3.
	aggregateUser := []map[string]any{
		{"user_id": 1, "follower_count": 0, "following_count": 1},
		{"user_id": 2, "follower_count": 5000, "following_count": 10},
		{"user_id": 3, "follower_count": 100, "following_count": 50},
		{"user_id": 4, "follower_count": 5000, "following_count": 10},
		{"user_id": 5, "follower_count": 5000, "following_count": 10},
		{"user_id": 6, "follower_count": 5000, "following_count": 10},
		{"user_id": 7, "follower_count": 5000, "following_count": 10},
		{"user_id": 8, "follower_count": 5000, "following_count": 10},
		{"user_id": 9, "follower_count": 5000, "following_count": 10},
		{"user_id": 10, "follower_count": 5000, "following_count": 10},
	}

	tracks := []map[string]any{
		// track 100 is mine: excluded as an own upload.
		{"track_id": 100, "owner_id": 1, "title": "my own track", "genre": "Rock", "created_at": daysAgo(5)},
		{"track_id": 200, "owner_id": 2, "title": "big hit", "genre": "Rock", "created_at": daysAgo(5)},
		{"track_id": 300, "owner_id": 3, "title": "underground gem", "genre": "Rock", "created_at": daysAgo(5)},
		{"track_id": 400, "owner_id": 4, "title": "followed artist track", "genre": "Rock", "created_at": daysAgo(5)},
		{"track_id": 500, "owner_id": 5, "title": "deactivated owner", "genre": "Rock", "created_at": daysAgo(5)},
		{"track_id": 600, "owner_id": 6, "title": "already played", "genre": "Rock", "created_at": daysAgo(5)},
		{"track_id": 700, "owner_id": 7, "title": "already saved", "genre": "Rock", "created_at": daysAgo(5)},
		{"track_id": 800, "owner_id": 8, "title": "gated", "genre": "Rock", "created_at": daysAgo(5), "is_stream_gated": true},
		{"track_id": 900, "owner_id": 9, "title": "ancient", "genre": "Rock", "created_at": daysAgo(400)},
		{"track_id": 1000, "owner_id": 10, "title": "two tracks a", "genre": "Rock", "created_at": daysAgo(5)},
		{"track_id": 1001, "owner_id": 10, "title": "two tracks b", "genre": "Rock", "created_at": daysAgo(5)},
		{"track_id": 1100, "owner_id": 2, "title": "unlisted", "genre": "Rock", "created_at": daysAgo(5), "is_unlisted": true},
		{"track_id": 1200, "owner_id": 2, "title": "deleted", "genre": "Rock", "created_at": daysAgo(5), "is_delete": true},
	}

	// My listening history: a rock track by user 6, which both establishes
	// Rock as my affinity genre and makes track 600 an already-played
	// exclusion.
	plays := []map[string]any{
		{"id": 1, "user_id": 1, "play_item_id": 600, "created_at": daysAgo(1)},
	}

	saves := []map[string]any{
		{"user_id": 1, "save_item_id": 700, "save_type": "track"},
	}

	follows := []map[string]any{
		{"follower_user_id": 1, "followee_user_id": 4},
	}

	// Every candidate needs a trending row to be retrieved at all.
	trackTrendingScores := []map[string]any{}
	for _, id := range []int{100, 200, 300, 400, 500, 600, 700, 800, 900, 1000, 1001, 1100, 1200} {
		trackTrendingScores = append(trackTrendingScores, map[string]any{
			"track_id": id, "score": 1_000_000_000, "time_range": "week",
		})
	}

	// Engagement is what drives quality_score. The gaps here are wide enough
	// to survive the +/-15% week jitter where the tests assert on ordering.
	aggregateTrack := []map[string]any{
		{"track_id": 200, "save_count": 500, "repost_count": 200},
		{"track_id": 300, "save_count": 30, "repost_count": 10},
		{"track_id": 400, "save_count": 30, "repost_count": 10},
		{"track_id": 500, "save_count": 400, "repost_count": 100},
		{"track_id": 600, "save_count": 400, "repost_count": 100},
		{"track_id": 700, "save_count": 400, "repost_count": 100},
		{"track_id": 800, "save_count": 400, "repost_count": 100},
		{"track_id": 900, "save_count": 400, "repost_count": 100},
		{"track_id": 1000, "save_count": 20, "repost_count": 5},
		{"track_id": 1001, "save_count": 20, "repost_count": 5},
	}

	return database.FixtureMap{
		"users":                 users,
		"aggregate_user":        aggregateUser,
		"tracks":                tracks,
		"plays":                 plays,
		"saves":                 saves,
		"follows":               follows,
		"track_trending_scores": trackTrendingScores,
		"aggregate_track":       aggregateTrack,
	}
}

// titles pulls the track titles out of a response, in order.
func discoverWeeklyTitles(tracks []dbv1.Track) []string {
	out := make([]string, len(tracks))
	for i, t := range tracks {
		out[i] = t.Title.String
	}
	return out
}

func TestV1UsersDiscoverWeekly(t *testing.T) {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], discoverWeeklyFixtures())

	var resp struct {
		Data []dbv1.Track
	}

	status, _ := testGet(t, app, "/v1/users/7eP5n/discover-weekly", &resp)
	assert.Equal(t, 200, status)

	titles := discoverWeeklyTitles(resp.Data)

	// Every filter, asserted as absence rather than ordering so the week
	// jitter can't make this flaky.
	assert.NotContains(t, titles, "my own track", "own uploads are excluded")
	assert.NotContains(t, titles, "deactivated owner", "deactivated owners are excluded")
	assert.NotContains(t, titles, "already played", "played tracks are excluded")
	assert.NotContains(t, titles, "already saved", "saved tracks are excluded")
	assert.NotContains(t, titles, "gated", "stream-gated tracks are excluded")
	assert.NotContains(t, titles, "ancient", "tracks past the age cutoff are excluded")
	assert.NotContains(t, titles, "unlisted", "unlisted tracks are excluded")
	assert.NotContains(t, titles, "deleted", "deleted tracks are excluded")

	// The survivors: users 2, 3, 4, and exactly one of user 10's two tracks.
	assert.Contains(t, titles, "big hit")
	assert.Contains(t, titles, "underground gem")
	assert.Contains(t, titles, "followed artist track")
	assert.Len(t, resp.Data, 4, "one track per artist across users 2, 3, 4, 10")

	// One-per-artist: user 10 uploaded two equally-scored tracks and gets
	// exactly one slot.
	twoTrackCount := 0
	for _, title := range titles {
		if title == "two tracks a" || title == "two tracks b" {
			twoTrackCount++
		}
	}
	assert.Equal(t, 1, twoTrackCount, "an artist never occupies two slots")
}

// A followed artist is demoted, not removed. The discovery weight gap
// (0.70 vs 1.25, a 1.79x ratio) is wider than the jitter band can close
// (1.35x at the extremes), so this ordering is guaranteed rather than
// merely likely.
func TestV1UsersDiscoverWeeklyDemotesFollowedArtists(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "me", "handle_lc": "me", "wallet": "0x0000000000000000000000000000000000000001"},
			{"user_id": 2, "handle": "followed", "handle_lc": "followed", "wallet": "0x0000000000000000000000000000000000000002"},
			{"user_id": 3, "handle": "stranger", "handle_lc": "stranger", "wallet": "0x0000000000000000000000000000000000000003"},
		},
		"aggregate_user": []map[string]any{
			{"user_id": 1, "follower_count": 0, "following_count": 1},
			{"user_id": 2, "follower_count": 5000, "following_count": 10},
			{"user_id": 3, "follower_count": 5000, "following_count": 10},
		},
		// Identical engagement and genre, so discovery_weight is the only
		// thing separating them.
		"tracks": []map[string]any{
			{"track_id": 200, "owner_id": 2, "title": "followed track", "genre": "Rock"},
			{"track_id": 300, "owner_id": 3, "title": "stranger track", "genre": "Rock"},
		},
		"aggregate_track": []map[string]any{
			{"track_id": 200, "save_count": 100, "repost_count": 50},
			{"track_id": 300, "save_count": 100, "repost_count": 50},
		},
		"follows": []map[string]any{
			{"follower_user_id": 1, "followee_user_id": 2},
		},
		"track_trending_scores": []map[string]any{
			{"track_id": 200, "score": 1_000_000_000, "time_range": "week"},
			{"track_id": 300, "score": 1_000_000_000, "time_range": "week"},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/users/7eP5n/discover-weekly", &resp)
	assert.Equal(t, 200, status)
	require.Len(t, resp.Data, 2)

	assert.Equal(t, "stranger track", resp.Data[0].Title.String,
		"an artist I don't follow outranks one I do, all else equal")
	assert.Equal(t, "followed track", resp.Data[1].Title.String,
		"but the followed artist is demoted, not filtered out")
}

// A listener with no plays, saves, follows, or reposts still gets a mix.
// This is the case that separates the surface from suggested-follows, which
// correctly returns nothing for a cold account: a mix that is empty on
// first open has no reason to exist.
func TestV1UsersDiscoverWeeklyColdStart(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "me", "handle_lc": "me", "wallet": "0x0000000000000000000000000000000000000001"},
			{"user_id": 2, "handle": "artist", "handle_lc": "artist", "wallet": "0x0000000000000000000000000000000000000002"},
		},
		"aggregate_user": []map[string]any{
			{"user_id": 1, "follower_count": 0, "following_count": 0},
			{"user_id": 2, "follower_count": 5000, "following_count": 10},
		},
		"tracks": []map[string]any{
			{"track_id": 200, "owner_id": 2, "title": "a track", "genre": "Rock"},
		},
		"aggregate_track": []map[string]any{
			{"track_id": 200, "save_count": 100, "repost_count": 50},
		},
		"track_trending_scores": []map[string]any{
			{"track_id": 200, "score": 1_000_000_000, "time_range": "week"},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/users/7eP5n/discover-weekly", &resp)
	assert.Equal(t, 200, status)
	assert.Len(t, resp.Data, 1, "no listening history still yields a mix")
	assert.Equal(t, "a track", resp.Data[0].Title.String)
}

// The mix must not move within a week and must move between weeks. Both
// halves matter: the first is the product promise, the second is the only
// thing keeping the mix from being the same 30 tracks forever.
//
// Goes through getDiscoverWeeklyTrackIds rather than the HTTP handler
// because the handler derives the period from the wall clock, and the point
// here is to vary it.
func TestV1UsersDiscoverWeeklyStableWithinWeek(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users":                 []map[string]any{{"user_id": 1, "handle": "me", "handle_lc": "me", "wallet": "0x0000000000000000000000000000000000000001"}},
		"aggregate_user":        []map[string]any{{"user_id": 1, "follower_count": 0, "following_count": 0}},
		"tracks":                []map[string]any{},
		"aggregate_track":       []map[string]any{},
		"track_trending_scores": []map[string]any{},
	}
	// A pool of equally-strong candidates by distinct artists. Equal scores
	// mean the week seed is the only thing deciding the order, which is
	// exactly what this test is about.
	for i := 0; i < 40; i++ {
		userId := 100 + i
		trackId := 1000 + i
		fixtures["users"] = append(fixtures["users"], map[string]any{
			"user_id": userId,
			"handle":  string(rune('a'+i%26)) + string(rune('a'+i/26)) + "artist",
			"wallet":  "0x" + padWallet(userId),
		})
		fixtures["aggregate_user"] = append(fixtures["aggregate_user"], map[string]any{
			"user_id": userId, "follower_count": 5000, "following_count": 10,
		})
		fixtures["tracks"] = append(fixtures["tracks"], map[string]any{
			"track_id": trackId, "owner_id": userId, "title": "track", "genre": "Rock",
		})
		fixtures["aggregate_track"] = append(fixtures["aggregate_track"], map[string]any{
			"track_id": trackId, "save_count": 100, "repost_count": 50,
		})
		fixtures["track_trending_scores"] = append(fixtures["track_trending_scores"], map[string]any{
			"track_id": trackId, "score": 1_000_000_000, "time_range": "week",
		})
	}
	// handle_lc is required alongside handle.
	for _, u := range fixtures["users"] {
		if h, ok := u["handle"].(string); ok {
			u["handle_lc"] = h
		}
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	ctx := context.Background()

	weekA1, err := app.getDiscoverWeeklyTrackIds(ctx, 1, 2026, 10, 20)
	require.NoError(t, err)
	require.NotEmpty(t, weekA1)

	// Same period, recomputed: byte-identical.
	app.discoverWeeklyCache.Clear()
	weekA2, err := app.getDiscoverWeeklyTrackIds(ctx, 1, 2026, 10, 20)
	require.NoError(t, err)
	assert.Equal(t, weekA1, weekA2,
		"the mix is deterministic for a given (user, year, week)")

	// Next week: same candidates, different mix.
	weekB, err := app.getDiscoverWeeklyTrackIds(ctx, 1, 2026, 11, 20)
	require.NoError(t, err)
	require.NotEmpty(t, weekB)
	assert.NotEqual(t, weekA1, weekB,
		"the week seed rotates the mix when the week rolls over")

	// And a different listener gets a different mix in the same week.
	weekAOther, err := app.getDiscoverWeeklyTrackIds(ctx, 2, 2026, 10, 20)
	require.NoError(t, err)
	assert.NotEqual(t, weekA1, weekAOther,
		"the seed is per-listener, not global")
}

// padWallet builds a distinct 40-hex-char wallet suffix from an int.
func padWallet(n int) string {
	const hexDigits = "0123456789abcdef"
	out := make([]byte, 40)
	for i := range out {
		out[i] = '0'
	}
	for i := len(out) - 1; i >= 0 && n > 0; i-- {
		out[i] = hexDigits[n%16]
		n /= 16
	}
	return string(out)
}

// The path :userId goes through requireUserIdMiddleware, so a junk hash id
// is a 400 rather than a silent fallback to user 0.
func TestV1UsersDiscoverWeeklyRequiresValidUserId(t *testing.T) {
	app := emptyTestApp(t)
	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/users/not-a-real-id/discover-weekly", &resp)
	assert.Equal(t, 400, status)
}
