package challenges

import (
	"fmt"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestTrackUpload_CompletesAt3(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{
		"blocks": {
			{"blockhash": "blk_25346437", "number": 25346437},
			{"blockhash": "blk_25346438", "number": 25346438},
			{"blockhash": "blk_25346439", "number": 25346439},
			{"blockhash": "blk_25346440", "number": 25346440},
		},
		"users": {{"user_id": 1, "wallet": "0x01"}, {"user_id": 2, "wallet": "0x02"}},
		"tracks": {
			{"track_id": 10, "owner_id": 1, "title": "T1", "blocknumber": 25346437, "created_at": time.Now()},
			{"track_id": 11, "owner_id": 1, "title": "T2", "blocknumber": 25346438, "created_at": time.Now()},
			{"track_id": 12, "owner_id": 1, "title": "T3", "blocknumber": 25346439, "created_at": time.Now()},
			{"track_id": 20, "owner_id": 2, "title": "T4", "blocknumber": 25346440, "created_at": time.Now()},
		},
	})

	runProcessor(t, pool, &TrackUploadProcessor{})

	r1, ok := queryUserChallenge(t, pool, "u", fmt.Sprintf("%x", 1))
	if assert.True(t, ok, "user 1 should have an u challenge row") {
		assert.True(t, r1.IsComplete, "user 1 has 3 tracks → complete")
		assert.Equal(t, int32(3), *r1.CurrentStepCount)
	}

	r2, ok := queryUserChallenge(t, pool, "u", fmt.Sprintf("%x", 2))
	if assert.True(t, ok, "user 2 should have an u row") {
		assert.False(t, r2.IsComplete, "user 2 only has 1 track")
		assert.Equal(t, int32(1), *r2.CurrentStepCount)
	}
}

func TestTrackUpload_IgnoresStemsAndUnlisted(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{
		"blocks": {
			{"blockhash": "blk_25346437", "number": 25346437},
			{"blockhash": "blk_25346438", "number": 25346438},
			{"blockhash": "blk_25346439", "number": 25346439},
			{"blockhash": "blk_25346440", "number": 25346440},
		},
		"users": {{"user_id": 3, "wallet": "0x03"}},
		"tracks": {
			{"track_id": 30, "owner_id": 3, "title": "Public", "blocknumber": 25346437},
			{"track_id": 31, "owner_id": 3, "title": "Unlisted", "blocknumber": 25346437, "is_unlisted": true},
			{"track_id": 32, "owner_id": 3, "title": "Stem", "blocknumber": 25346437, "stem_of": `{"parent_track_id": 30}`},
		},
	})

	runProcessor(t, pool, &TrackUploadProcessor{})

	r, ok := queryUserChallenge(t, pool, "u", fmt.Sprintf("%x", 3))
	if assert.True(t, ok) {
		assert.Equal(t, int32(1), *r.CurrentStepCount, "only the public, non-stem track counts")
		assert.False(t, r.IsComplete)
	}
}
