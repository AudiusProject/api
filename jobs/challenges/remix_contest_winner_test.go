package challenges

import (
	"context"
	"fmt"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRemixContestWinner_VerifiedHostMintsRows — happy path: verified
// host's remix contest with two winner track IDs → two user_challenges
// rows, one per remixer.
func TestRemixContestWinner_VerifiedHostMintsRows(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_w", "number": 1}},
		"users": {
			{"user_id": 1300, "wallet": "0x1300", "is_verified": true},  // host
			{"user_id": 1301, "wallet": "0x1301", "is_verified": false}, // remixer A
			{"user_id": 1302, "wallet": "0x1302", "is_verified": false}, // remixer B
		},
		"tracks": {
			{"track_id": 130001, "owner_id": 1301, "title": "Remix A", "blocknumber": 1},
			{"track_id": 130002, "owner_id": 1302, "title": "Remix B", "blocknumber": 1},
		},
	})

	// events table — schema in api/sql/01_schema.sql.
	_, err := pool.Exec(ctx, `
		INSERT INTO events (event_id, event_type, user_id, entity_type, entity_id, is_deleted, event_data, blocknumber, txhash, blockhash)
		VALUES (50000, 'remix_contest', 1300, 'track', 130000, false, $1::jsonb, 1, 'tx-e', 'bh-e')
	`, `{"winners": [130001, 130002]}`)
	require.NoError(t, err)

	runProcessor(t, pool, &RemixContestWinnerProcessor{})

	for _, winner := range []struct {
		userID, trackID int
	}{{1301, 130001}, {1302, 130002}} {
		spec := fmt.Sprintf("%x:%x", 50000, winner.userID)
		r, ok := queryUserChallenge(t, pool, "w", spec)
		if assert.True(t, ok, "user %d should win remix contest", winner.userID) {
			assert.True(t, r.IsComplete)
			assert.Equal(t, int32(1000), r.Amount)
			assert.Equal(t, int64(winner.userID), r.UserID)
		}
	}
}

func TestRemixContestWinner_UnverifiedHostSkipped(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_w2", "number": 1}},
		"users": {
			{"user_id": 1310, "wallet": "0x1310", "is_verified": false}, // unverified host
			{"user_id": 1311, "wallet": "0x1311"},
		},
		"tracks": {{"track_id": 131001, "owner_id": 1311, "title": "Remix", "blocknumber": 1}},
	})
	_, err := pool.Exec(ctx, `
		INSERT INTO events (event_id, event_type, user_id, entity_type, entity_id, is_deleted, event_data, blocknumber, txhash, blockhash)
		VALUES (50010, 'remix_contest', 1310, 'track', 131000, false, $1::jsonb, 1, 'tx', 'bh')
	`, `{"winners": [131001]}`)
	require.NoError(t, err)

	runProcessor(t, pool, &RemixContestWinnerProcessor{})

	_, ok := queryUserChallenge(t, pool, "w", fmt.Sprintf("%x:%x", 50010, 1311))
	assert.False(t, ok, "unverified host should produce no rows")
}
