package challenges

import (
	"fmt"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestFirstPlaylist_CompletesOnAnyPlaylist(t *testing.T) {
	pool := withChallengesDB(t)
	database.Seed(pool, database.FixtureMap{
		"blocks":    {{"blockhash": "blk_28350001", "number": 28350001}},
		"users":     {{"user_id": 100, "wallet": "0x100"}, {"user_id": 101, "wallet": "0x101"}},
		"playlists": {{"playlist_id": 1, "playlist_owner_id": 100, "blocknumber": 28350001}},
	})

	runProcessor(t, pool, &FirstPlaylistProcessor{})

	r1, ok := queryUserChallenge(t, pool, "fp", fmt.Sprintf("%x", 100))
	if assert.True(t, ok) {
		assert.True(t, r1.IsComplete)
		assert.Equal(t, int32(2), r1.Amount, "amount=2 per challenges.json")
	}
	_, ok = queryUserChallenge(t, pool, "fp", fmt.Sprintf("%x", 101))
	assert.False(t, ok, "user 101 has no playlist; no row")
}
