package challenges

import (
	"fmt"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProfileCompletion_PartialProgress(t *testing.T) {
	pool := withChallengesDB(t)

	// User with bio + name + picture (3 of 7 steps) — incomplete.
	database.Seed(pool, database.FixtureMap{
		"users": {{
			"user_id":         200,
			"handle":          "u200",
			"handle_lc":       "u200",
			"wallet":          "0x200",
			"bio":             "hi",
			"name":            "U Two Hundred",
			"profile_picture": "QmXYZ",
		}},
	})

	runProcessor(t, pool, &ProfileCompletionProcessor{})

	r, ok := queryUserChallenge(t, pool, "p", fmt.Sprintf("%x", 200))
	require.True(t, ok)
	assert.Equal(t, int32(3), *r.CurrentStepCount)
	assert.False(t, r.IsComplete, "3/7 → incomplete")
}

func TestProfileCompletion_FullyComplete(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{
		"users": {{
			"user_id":         210,
			"handle":          "u210",
			"handle_lc":       "u210",
			"wallet":          "0x210",
			"bio":             "hi",
			"name":            "User",
			"profile_picture": "QmA",
			"cover_photo":     "QmB",
		}, {"user_id": 211, "wallet": "0x211", "handle": "f1", "handle_lc": "f1"},
			{"user_id": 212, "wallet": "0x212", "handle": "f2", "handle_lc": "f2"},
			{"user_id": 213, "wallet": "0x213", "handle": "f3", "handle_lc": "f3"},
			{"user_id": 214, "wallet": "0x214", "handle": "f4", "handle_lc": "f4"},
			{"user_id": 215, "wallet": "0x215", "handle": "f5", "handle_lc": "f5"},
			{"user_id": 220, "wallet": "0x220", "handle": "target", "handle_lc": "target"}},
		"follows": {
			{"follower_user_id": 210, "followee_user_id": 211, "is_current": true, "is_delete": false},
			{"follower_user_id": 210, "followee_user_id": 212, "is_current": true, "is_delete": false},
			{"follower_user_id": 210, "followee_user_id": 213, "is_current": true, "is_delete": false},
			{"follower_user_id": 210, "followee_user_id": 214, "is_current": true, "is_delete": false},
			{"follower_user_id": 210, "followee_user_id": 215, "is_current": true, "is_delete": false},
		},
		"reposts": {
			{"user_id": 210, "repost_item_id": 1, "repost_type": "track", "is_current": true, "is_delete": false},
		},
		"saves": {
			{"user_id": 210, "save_item_id": 1, "save_type": "track", "is_current": true, "is_delete": false},
		},
	})

	runProcessor(t, pool, &ProfileCompletionProcessor{})

	r, ok := queryUserChallenge(t, pool, "p", fmt.Sprintf("%x", 210))
	require.True(t, ok)
	assert.Equal(t, int32(7), *r.CurrentStepCount)
	assert.True(t, r.IsComplete)
}
