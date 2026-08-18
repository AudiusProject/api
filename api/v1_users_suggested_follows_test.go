package api

import (
	"testing"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersSuggestedFollows(t *testing.T) {
	app := emptyTestApp(t)

	// user 1 is the seed user. Every other user owns content user 1 has
	// engaged with, or is a control for one of the exclusion rules.
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "rayjacobson", "handle_lc": "rayjacobson", "name": "Ray Jacobson", "wallet": "0x7d273271690538cf855e5b3002a0dd8c154bb060"},
			{"user_id": 2, "handle": "twofaves", "handle_lc": "twofaves", "wallet": "0x0000000000000000000000000000000000000002"},
			{"user_id": 3, "handle": "onerepost", "handle_lc": "onerepost", "wallet": "0x0000000000000000000000000000000000000003"},
			{"user_id": 4, "handle": "alreadyfollowed", "handle_lc": "alreadyfollowed", "wallet": "0x0000000000000000000000000000000000000004"},
			{"user_id": 5, "handle": "deactivated", "handle_lc": "deactivated", "is_deactivated": true, "wallet": "0x0000000000000000000000000000000000000005"},
			{"user_id": 6, "handle": "onefave", "handle_lc": "onefave", "wallet": "0x0000000000000000000000000000000000000006"},
			{"user_id": 7, "handle": "albumowner", "handle_lc": "albumowner", "wallet": "0x0000000000000000000000000000000000000007"},
			{"user_id": 8, "handle": "unlistedonly", "handle_lc": "unlistedonly", "wallet": "0x0000000000000000000000000000000000000008"},
		},
		"aggregate_user": []map[string]any{
			{"user_id": 1, "follower_count": 10},
			{"user_id": 2, "follower_count": 500},
			{"user_id": 3, "follower_count": 400},
			{"user_id": 4, "follower_count": 300},
			{"user_id": 5, "follower_count": 200},
			{"user_id": 6, "follower_count": 100},
			{"user_id": 7, "follower_count": 50},
			{"user_id": 8, "follower_count": 25},
		},
		"tracks": []map[string]any{
			{"track_id": 100, "owner_id": 1, "title": "my own track"},
			{"track_id": 200, "owner_id": 2, "title": "faved a"},
			{"track_id": 201, "owner_id": 2, "title": "faved b"},
			{"track_id": 300, "owner_id": 3, "title": "reposted"},
			{"track_id": 400, "owner_id": 4, "title": "faved but followed"},
			{"track_id": 500, "owner_id": 5, "title": "faved but deactivated"},
			{"track_id": 600, "owner_id": 6, "title": "faved once"},
			{"track_id": 800, "owner_id": 8, "title": "unlisted", "is_unlisted": true},
		},
		"playlists": []map[string]any{
			{"playlist_id": 700, "playlist_owner_id": 7, "playlist_name": "an album", "is_album": true},
		},
		"follows": []map[string]any{
			{"follower_user_id": 1, "followee_user_id": 4},
		},
		"saves": []map[string]any{
			{"user_id": 1, "save_item_id": 100, "save_type": "track"}, // self, excluded
			{"user_id": 1, "save_item_id": 200, "save_type": "track"},
			{"user_id": 1, "save_item_id": 201, "save_type": "track"},
			{"user_id": 1, "save_item_id": 400, "save_type": "track"}, // already followed, excluded
			{"user_id": 1, "save_item_id": 500, "save_type": "track"}, // deactivated, excluded
			{"user_id": 1, "save_item_id": 600, "save_type": "track"},
			{"user_id": 1, "save_item_id": 700, "save_type": "album"},
			{"user_id": 1, "save_item_id": 800, "save_type": "track"}, // unlisted track, excluded
		},
		"reposts": []map[string]any{
			{"user_id": 1, "repost_item_id": 300, "repost_type": "track"},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	var resp struct {
		Data []dbv1.User
	}

	// Ranking: user 2 has two favorites (2.0) > user 3's single repost (1.5) >
	// the two single favorites (1.0 each), which tie and fall back to user_id.
	{
		status, _ := testGet(t, app, "/v1/users/7eP5n/suggested-follows", &resp)
		assert.Equal(t, 200, status)
		handles := make([]string, len(resp.Data))
		for i, u := range resp.Data {
			handles[i] = u.Handle.String
		}
		assert.Equal(t, []string{"twofaves", "onerepost", "onefave", "albumowner"}, handles)
	}

	// A repost outweighs a single favorite even though both users were engaged
	// with exactly once.
	{
		status, _ := testGet(t, app, "/v1/users/7eP5n/suggested-follows?limit=2", &resp)
		assert.Equal(t, 200, status)
		assert.Len(t, resp.Data, 2)
		assert.Equal(t, "twofaves", resp.Data[0].Handle.String)
		assert.Equal(t, "onerepost", resp.Data[1].Handle.String)
	}

	// Offset walks the same ordering rather than reshuffling it.
	{
		status, _ := testGet(t, app, "/v1/users/7eP5n/suggested-follows?limit=2&offset=2", &resp)
		assert.Equal(t, 200, status)
		assert.Len(t, resp.Data, 2)
		assert.Equal(t, "onefave", resp.Data[0].Handle.String)
		assert.Equal(t, "albumowner", resp.Data[1].Handle.String)
	}

	// A user with no favorites or reposts gets nothing rather than an error --
	// the caller is expected to fall back to a non-personalized surface.
	{
		status, _ := testGet(t, app, "/v1/users/ML51L/suggested-follows", &resp)
		assert.Equal(t, 200, status)
		assert.Len(t, resp.Data, 0)
	}
}

// The decay term is the only reason a single favorite can outrank another
// single favorite, so it needs a case of its own -- the fixtures above all
// share a created_at and would pass with the decay removed entirely.
func TestV1UsersSuggestedFollowsRecencyDecay(t *testing.T) {
	app := emptyTestApp(t)

	longAgo := time.Now().AddDate(-2, 0, 0)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "rayjacobson", "handle_lc": "rayjacobson", "wallet": "0x7d273271690538cf855e5b3002a0dd8c154bb060"},
			{"user_id": 2, "handle": "stale", "handle_lc": "stale", "wallet": "0x0000000000000000000000000000000000000002"},
			{"user_id": 3, "handle": "fresh", "handle_lc": "fresh", "wallet": "0x0000000000000000000000000000000000000003"},
		},
		"aggregate_user": []map[string]any{
			{"user_id": 1, "follower_count": 10},
			{"user_id": 2, "follower_count": 10},
			{"user_id": 3, "follower_count": 10},
		},
		"tracks": []map[string]any{
			{"track_id": 200, "owner_id": 2, "title": "old favorite"},
			{"track_id": 300, "owner_id": 3, "title": "new favorite"},
		},
		"saves": []map[string]any{
			{"user_id": 1, "save_item_id": 200, "save_type": "track", "created_at": longAgo},
			{"user_id": 1, "save_item_id": 300, "save_type": "track", "created_at": time.Now()},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	var resp struct {
		Data []dbv1.User
	}

	// Equal raw engagement (one favorite each), so recency alone decides. User 2
	// sorts first on user_id, which makes this fail loudly if decay stops working.
	status, _ := testGet(t, app, "/v1/users/7eP5n/suggested-follows", &resp)
	assert.Equal(t, 200, status)
	assert.Len(t, resp.Data, 2)
	assert.Equal(t, "fresh", resp.Data[0].Handle.String)
	assert.Equal(t, "stale", resp.Data[1].Handle.String)
}
