package challenges

import (
	"fmt"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestProfileVerified_CompletesForVerifiedUsers(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{
		"users": {
			{"user_id": 300, "wallet": "0x300", "is_verified": true},
			{"user_id": 301, "wallet": "0x301", "is_verified": false},
		},
	})

	runProcessor(t, pool, &ProfileVerifiedProcessor{})

	r, ok := queryUserChallenge(t, pool, "v", fmt.Sprintf("%x", 300))
	if assert.True(t, ok) {
		assert.True(t, r.IsComplete)
		assert.Equal(t, int32(5), r.Amount)
	}
	_, ok = queryUserChallenge(t, pool, "v", fmt.Sprintf("%x", 301))
	assert.False(t, ok, "unverified user has no row")
}
