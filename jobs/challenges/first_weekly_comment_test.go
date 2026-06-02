package challenges

import (
	"fmt"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

// TestFirstWeeklyComment_OneRowPerUserPerWeek verifies that:
//   - a single comment in a week mints one user_challenge row
//   - multiple comments in the same week still only produce one row
//   - comments in different ISO weeks produce different rows
func TestFirstWeeklyComment_OneRowPerUserPerWeek(t *testing.T) {
	pool := withChallengesDB(t)

	// Two timestamps in the same ISO week, plus one a few weeks later.
	wk1A := time.Date(2025, 1, 6, 12, 0, 0, 0, time.UTC) // Mon Jan 6 = ISO week 02 of 2025
	wk1B := time.Date(2025, 1, 8, 12, 0, 0, 0, time.UTC) // Wed Jan 8 — same week
	wk5 := time.Date(2025, 2, 5, 12, 0, 0, 0, time.UTC)  // Wed Feb 5 = ISO week 06

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_fwc", "number": 1}},
		"users":  {{"user_id": 700, "wallet": "0x700"}, {"user_id": 800, "wallet": "0x800"}},
		"tracks": {{"track_id": 7000, "owner_id": 800, "title": "T", "blocknumber": 1}},
		// blocknumber must be > 0 so the incremental dirty scan picks these up
		// (tests don't run migrations, so the checkpoint stays at 0).
		"comments": {
			{"comment_id": 1, "user_id": 700, "entity_id": 7000, "entity_type": "Track", "text": "a", "created_at": wk1A, "blockhash": "x", "txhash": "tx1", "blocknumber": 1},
			{"comment_id": 2, "user_id": 700, "entity_id": 7000, "entity_type": "Track", "text": "b", "created_at": wk1B, "blockhash": "x", "txhash": "tx2", "blocknumber": 1},
			{"comment_id": 3, "user_id": 700, "entity_id": 7000, "entity_type": "Track", "text": "c", "created_at": wk5, "blockhash": "x", "txhash": "tx3", "blocknumber": 1},
		},
	})

	runProcessor(t, pool, &FirstWeeklyCommentProcessor{})

	// One row for ISO week 02/2025
	_, ok := queryUserChallenge(t, pool, "c", fmt.Sprintf("%x:%d%02d", 700, 2025, 2))
	assert.True(t, ok, "expected row for 2025 week 02")

	// One row for ISO week 06/2025
	_, ok = queryUserChallenge(t, pool, "c", fmt.Sprintf("%x:%d%02d", 700, 2025, 6))
	assert.True(t, ok, "expected row for 2025 week 06")

	// No row for week 03 (no comments).
	_, ok = queryUserChallenge(t, pool, "c", fmt.Sprintf("%x:%d%02d", 700, 2025, 3))
	assert.False(t, ok)
}
