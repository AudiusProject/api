package challenges

import (
	"fmt"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestCommentPin_VerifiedOwnerPinsOthersComment(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_cp", "number": 1}},
		"users": {
			{"user_id": 900, "wallet": "0x900", "is_verified": true},  // verified track owner
			{"user_id": 901, "wallet": "0x901", "is_verified": false}, // commenter
		},
		"tracks": {{"track_id": 9000, "owner_id": 900, "title": "T", "blocknumber": 1, "pinned_comment_id": 1}},
		"comments": {
			{"comment_id": 1, "user_id": 901, "entity_id": 9000, "entity_type": "Track", "text": "nice!", "blockhash": "x", "txhash": "tx"},
		},
	})

	runProcessor(t, pool, &CommentPinProcessor{})

	r, ok := queryUserChallenge(t, pool, "cp", fmt.Sprintf("%x:%x", 901, 9000))
	if assert.True(t, ok) {
		assert.True(t, r.IsComplete)
		assert.Equal(t, int32(10), r.Amount)
		assert.Equal(t, int64(901), r.UserID, "row should be filed under commenter")
	}
}

func TestCommentPin_SkippedWhenOwnerNotVerified(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_cp2", "number": 1}},
		"users": {
			{"user_id": 910, "wallet": "0x910", "is_verified": false}, // owner not verified
			{"user_id": 911, "wallet": "0x911"},
		},
		"tracks":   {{"track_id": 9100, "owner_id": 910, "title": "T", "blocknumber": 1, "pinned_comment_id": 1}},
		"comments": {{"comment_id": 1, "user_id": 911, "entity_id": 9100, "entity_type": "Track", "text": "x", "blockhash": "x", "txhash": "tx"}},
	})

	runProcessor(t, pool, &CommentPinProcessor{})

	_, ok := queryUserChallenge(t, pool, "cp", fmt.Sprintf("%x:%x", 911, 9100))
	assert.False(t, ok)
}

func TestCommentPin_SkippedForSelfPin(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{
		"blocks":   {{"blockhash": "blk_cp3", "number": 1}},
		"users":    {{"user_id": 920, "wallet": "0x920", "is_verified": true}},
		"tracks":   {{"track_id": 9200, "owner_id": 920, "title": "T", "blocknumber": 1, "pinned_comment_id": 1}},
		"comments": {{"comment_id": 1, "user_id": 920, "entity_id": 9200, "entity_type": "Track", "text": "self", "blockhash": "x", "txhash": "tx"}},
	})

	runProcessor(t, pool, &CommentPinProcessor{})

	_, ok := queryUserChallenge(t, pool, "cp", fmt.Sprintf("%x:%x", 920, 9200))
	assert.False(t, ok, "self-pin should not earn the challenge")
}
