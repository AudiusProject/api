package api

import (
	"context"
	"testing"

	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

// No reports/mutes/deletes
func TestGetTrackCommentCount(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := FixtureMap{
		"tracks": []map[string]any{
			{"track_id": 1, "owner_id": 1},
		},
		"users": []map[string]any{
			{"user_id": 1, "wallet": "0x7d273271690538cf855e5b3002a0dd8c154bb060"},
			{"user_id": 2, "wallet": "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"},
			{"user_id": 3},
			{"user_id": 4},
		},
		"comments": []map[string]any{
			{"comment_id": 1, "user_id": 2, "entity_id": 1, "entity_type": "Track"},
			{"comment_id": 2, "user_id": 3, "entity_id": 1, "entity_type": "Track"},
			{"comment_id": 3, "user_id": 4, "entity_id": 1, "entity_type": "Track"},
		},
	}
	createFixtures(app, fixtures)

	// Check count for user 1
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(1),
			"0x7d273271690538cf855e5b3002a0dd8c154bb060",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 3,
		})
	}

	// Check count for anonymous user
	{
		status, body := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 3,
		})
	}
}

// Test that we don't count comments reported by the user
func TestGetTrackCommentCountUserReportedComment(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := FixtureMap{
		"tracks": []map[string]any{
			{"track_id": 1, "owner_id": 1},
		},
		"users": []map[string]any{
			{"user_id": 1, "wallet": "0x7d273271690538cf855e5b3002a0dd8c154bb060"},
			{"user_id": 2, "wallet": "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"},
			{"user_id": 3},
			{"user_id": 4},
		},
		"comments": []map[string]any{
			{"comment_id": 1, "user_id": 2, "entity_id": 1, "entity_type": "Track"},
			{"comment_id": 2, "user_id": 3, "entity_id": 1, "entity_type": "Track"},
			{"comment_id": 3, "user_id": 4, "entity_id": 1, "entity_type": "Track"},
		},
		"comment_reports": []map[string]any{
			{"comment_id": 1, "user_id": 2}, // User 2 reported comment 1
		},
	}
	createFixtures(app, fixtures)

	// Check count for user 2
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(2),
			"0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 2, // Should not see the comment they reported
		})
	}

	// Check count for track owner
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(1),
			"0x7d273271690538cf855e5b3002a0dd8c154bb060",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 3,
		})
	}

	// Check count for anonymous user
	{
		status, body := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 3,
		})
	}
}

// Test that we don't count comments reported by the track owner
func TestGetTrackCommentCountArtistReportedComment(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := FixtureMap{
		"tracks": []map[string]any{
			{"track_id": 1, "owner_id": 1},
		},
		"users": []map[string]any{
			{"user_id": 1, "wallet": "0x7d273271690538cf855e5b3002a0dd8c154bb060"},
			{"user_id": 2, "wallet": "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"},
			{"user_id": 3},
			{"user_id": 4},
		},
		"comments": []map[string]any{
			{"comment_id": 1, "user_id": 2, "entity_id": 1, "entity_type": "Track"},
			{"comment_id": 2, "user_id": 3, "entity_id": 1, "entity_type": "Track"},
			{"comment_id": 3, "user_id": 4, "entity_id": 1, "entity_type": "Track"},
		},
		"comment_reports": []map[string]any{
			{"comment_id": 1, "user_id": 1}, // Reported by track owner
		},
	}
	createFixtures(app, fixtures)

	// Check count for anonymous user
	{
		status, body := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 2,
		})
	}

	// Check count for track owner
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(1),
			"0x7d273271690538cf855e5b3002a0dd8c154bb060",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 2,
		})
	}

	// Check count for user 2
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(2),
			"0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 2,
		})
	}
}

// Test that we don't count comments reported by a high-karma user
func TestGetTrackCommentCountWithKarmaReportedComment(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "wallet": "0x7d273271690538cf855e5b3002a0dd8c154bb060"},
			{"user_id": 2, "wallet": "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"},
			{"user_id": 3},
			{"user_id": 4},
		},
		"tracks": []map[string]any{
			{"track_id": 1, "owner_id": 1},
		},
		"comments": []map[string]any{
			{"comment_id": 1, "user_id": 2, "entity_id": 1, "entity_type": "Track"},
			{"comment_id": 2, "user_id": 3, "entity_id": 1, "entity_type": "Track"},
			{"comment_id": 3, "user_id": 4, "entity_id": 1, "entity_type": "Track"},
		},
		"comment_reports": []map[string]any{
			{"comment_id": 2, "user_id": 2}, // Reported by high-karma user
		},
	}
	createFixtures(app, fixtures)
	_, err := app.pool.Exec(context.Background(), `
		UPDATE aggregate_user SET follower_count = $1 WHERE user_id = $2
	`, karmaCommentCountThreshold+1, 2)
	if err != nil {
		t.Fatal(err)
	}

	// Check count for anonymous user
	{
		status, body := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 2,
		})
	}

	// Check count for user 1 (track owner)
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(1),
			"0x7d273271690538cf855e5b3002a0dd8c154bb060",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 2,
		})
	}

	// Check count for user 2 (high karma user who reported)
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(2),
			"0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 2,
		})
	}
}

// Test that a deleted comment is not counted
func TestGetTrackCommentCountDeletedComment(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := FixtureMap{
		"tracks": []map[string]any{
			{"track_id": 1, "owner_id": 1},
		},
		"users": []map[string]any{
			{"user_id": 1},
			{"user_id": 2},
			{"user_id": 3},
			{"user_id": 4},
		},
		"comments": []map[string]any{
			{"comment_id": 1, "user_id": 2, "entity_id": 1, "entity_type": "Track", "is_delete": false},
			{"comment_id": 2, "user_id": 3, "entity_id": 1, "entity_type": "Track", "is_delete": true},
			{"comment_id": 3, "user_id": 4, "entity_id": 1, "entity_type": "Track", "is_delete": false},
		},
	}
	createFixtures(app, fixtures)

	status, body := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data": 2,
	})
}

// Test that a muted user's comments are not counted for the muting user
func TestGetTrackCommentCountMutedUser(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := FixtureMap{
		"tracks": []map[string]any{
			{"track_id": 1, "owner_id": 1},
		},
		"users": []map[string]any{
			{"user_id": 1, "wallet": "0x7d273271690538cf855e5b3002a0dd8c154bb060"},
			{"user_id": 2, "wallet": "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"},
			{"user_id": 3},
			{"user_id": 4},
		},
		"comments": []map[string]any{
			{"comment_id": 1, "user_id": 2, "entity_id": 1, "entity_type": "Track"},
			{"comment_id": 2, "user_id": 3, "entity_id": 1, "entity_type": "Track"},
			{"comment_id": 3, "user_id": 3, "entity_id": 1, "entity_type": "Track"},
		},
		"muted_users": []map[string]any{
			{"user_id": 2, "muted_user_id": 3}, // User 2 mutes user 3
		},
	}
	createFixtures(app, fixtures)

	// For user 2 who muted user 3, should only see 1 comment
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(2),
			"0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 1,
		})
	}

	// For user 1 who hasn't muted anyone, should see all 3 comments
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(1),
			"0x7d273271690538cf855e5b3002a0dd8c154bb060",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 3,
		})
	}

	// Anonymous user should see all 3 comments
	{
		status, body := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 3,
		})
	}
}

// Test that when an artist mutes someone, their comments are not counted for everyone
func TestGetTrackCommentCountArtistMutedUser(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := FixtureMap{
		"tracks": []map[string]any{
			{"track_id": 1, "owner_id": 1}, // Artist
		},
		"users": []map[string]any{
			{"user_id": 1, "wallet": "0x7d273271690538cf855e5b3002a0dd8c154bb060"}, // Artist
			{"user_id": 2, "wallet": "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"}, // Muted user
			{"user_id": 3, "wallet": "0x4954d18926ba0ed9378938444731be4e622537b2"}, // Regular user
			{"user_id": 4}, // Another regular user
		},
		"comments": []map[string]any{
			{"comment_id": 1, "user_id": 2, "entity_id": 1, "entity_type": "Track"}, // muted comment
			{"comment_id": 2, "user_id": 2, "entity_id": 1, "entity_type": "Track"}, // muted comment
			{"comment_id": 3, "user_id": 4, "entity_id": 1, "entity_type": "Track"}, // regular comment
		},
		"muted_users": []map[string]any{
			{"user_id": 1, "muted_user_id": 2}, // Artist (user 1) mutes user 2
		},
	}
	createFixtures(app, fixtures)

	// The artist who muted should only see 1 comment
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(1),
			"0x7d273271690538cf855e5b3002a0dd8c154bb060",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 1,
		})
	}

	// User 3 should also only see 1 comment
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(3),
			"0x4954d18926ba0ed9378938444731be4e622537b2",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 1,
		})
	}

	// Anonymous user should also only see 1 comment
	{
		status, body := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 1,
		})
	}

	// Muted user should still see their own comments
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(2),
			"0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 3,
		})
	}
}

// Test that when a high karma user mutes someone, their comments are hidden for everyone
func TestGetTrackCommentCountHighKarmaMutedUser(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := FixtureMap{
		"tracks": []map[string]any{
			{"track_id": 1, "owner_id": 1},
		},
		"users": []map[string]any{
			{"user_id": 1, "wallet": "0x7d273271690538cf855e5b3002a0dd8c154bb060"}, // Track artist
			{"user_id": 2, "wallet": "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"}, // Muted user
			{"user_id": 3, "wallet": "0x4954d18926ba0ed9378938444731be4e622537b2"}, // High karma user
			{"user_id": 4}, // Regular user
		},
		"comments": []map[string]any{
			{"comment_id": 1, "user_id": 2, "entity_id": 1, "entity_type": "Track"}, // muted comment
			{"comment_id": 2, "user_id": 2, "entity_id": 1, "entity_type": "Track"}, // muted comment
			{"comment_id": 3, "user_id": 4, "entity_id": 1, "entity_type": "Track"}, // regular comment
		},
		"muted_users": []map[string]any{
			{"user_id": 3, "muted_user_id": 2}, // High karma user 3 mutes user 2
		},
	}
	createFixtures(app, fixtures)
	_, err := app.pool.Exec(context.Background(), `
		UPDATE aggregate_user SET follower_count = $1 WHERE user_id = $2
	`, karmaCommentCountThreshold+1, 3)
	if err != nil {
		t.Fatal(err)
	}

	// Track owner should only see 1 comment
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(1),
			"0x7d273271690538cf855e5b3002a0dd8c154bb060",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 1,
		})
	}

	// High karma user should only see 1 comment
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(3),
			"0x4954d18926ba0ed9378938444731be4e622537b2",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 1,
		})
	}

	// Anonymous user should only see 1 comment
	{
		status, body := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 1,
		})
	}

	// Muted user should still see their own comments
	{
		status, body := testGetWithWallet(
			t, app,
			"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_count?user_id="+trashid.MustEncodeHashID(2),
			"0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
		)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": 3,
		})
	}
}
