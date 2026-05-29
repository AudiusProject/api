package challenges

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTastemaker_EarliestRepostersAndSavers — for a track in the top-10
// trending, the earliest 10 reposters AND earliest 10 savers each earn a
// tastemaker row (deduped by specifier).
func TestTastemaker_EarliestRepostersAndSavers(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_tm", "number": 1}},
		"users": {
			{"user_id": 1200, "wallet": "0x1200"},
			{"user_id": 1201, "wallet": "0x1201"},
			{"user_id": 1202, "wallet": "0x1202"},
		},
		"tracks": {{"track_id": 120000, "owner_id": 1200, "title": "Hit", "blocknumber": 1}},
	})

	// Put the track in track_trending_scores (top of week range).
	_, err := pool.Exec(ctx, `
		INSERT INTO track_trending_scores (track_id, type, version, time_range, score, created_at)
		VALUES (120000, 'TRACKS', 'pnagD', 'week', 999.0, now())
	`)
	require.NoError(t, err)

	// 1201 reposts, 1202 saves.
	_, err = pool.Exec(ctx, `
		INSERT INTO reposts (user_id, repost_item_id, repost_type, is_current, is_delete, created_at, txhash)
		VALUES (1201, 120000, 'track', true, false, now() - interval '1 hour', 'tx-rp')
	`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO saves (user_id, save_item_id, save_type, is_current, is_delete, created_at, txhash)
		VALUES (1202, 120000, 'track', true, false, now() - interval '30 minutes', 'tx-sv')
	`)
	require.NoError(t, err)

	runProcessor(t, pool, &TastemakerProcessor{})

	for _, uid := range []int{1201, 1202} {
		spec := fmt.Sprintf("%x:t:%x", uid, 120000)
		r, ok := queryUserChallenge(t, pool, "t", spec)
		if assert.True(t, ok, "user %d should have tastemaker row", uid) {
			assert.True(t, r.IsComplete)
			assert.Equal(t, int32(100), r.Amount)
		}
	}
}

// TestTastemaker_EmitsNotification — handle_tastemaker trigger fans out
// a `tastemaker` notification when a user_challenges row is minted for
// challenge_id='t'. Repost wins over save when a user qualifies via both.
func TestTastemaker_EmitsNotification(t *testing.T) {
	pool := withChallengesDB(t)
	ctx := context.Background()

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_tmn", "number": 1}},
		"users": {
			{"user_id": 2200, "wallet": "0x2200"},
			{"user_id": 2201, "wallet": "0x2201"},
			{"user_id": 2202, "wallet": "0x2202"},
		},
		"tracks": {{"track_id": 220000, "owner_id": 2200, "title": "Hit", "blocknumber": 1}},
	})

	_, err := pool.Exec(ctx, `
		INSERT INTO track_trending_scores (track_id, type, version, time_range, score, created_at)
		VALUES (220000, 'TRACKS', 'pnagD', 'week', 999.0, now())
	`)
	require.NoError(t, err)

	// 2201 reposts only; 2202 both reposts AND saves (repost should win).
	_, err = pool.Exec(ctx, `
		INSERT INTO reposts (user_id, repost_item_id, repost_type, is_current, is_delete, created_at, txhash)
		VALUES
			(2201, 220000, 'track', true, false, now() - interval '2 hours', 'tx-rp-2201'),
			(2202, 220000, 'track', true, false, now() - interval '1 hour',  'tx-rp-2202')
	`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `
		INSERT INTO saves (user_id, save_item_id, save_type, is_current, is_delete, created_at, txhash)
		VALUES (2202, 220000, 'track', true, false, now() - interval '30 minutes', 'tx-sv-2202')
	`)
	require.NoError(t, err)

	runProcessor(t, pool, &TastemakerProcessor{})

	type notifRow struct {
		UserIDs   []int64
		Specifier string
		GroupID   string
		Data      []byte
	}
	queryNotif := func(uid int) (notifRow, bool) {
		var n notifRow
		err := pool.QueryRow(ctx, `
			SELECT user_ids, specifier, group_id, data
			FROM notification
			WHERE type = 'tastemaker'
			  AND group_id = $1
		`, fmt.Sprintf("tastemaker_user_id:%d:tastemaker_item_id:%d", uid, 220000)).
			Scan(&n.UserIDs, &n.Specifier, &n.GroupID, &n.Data)
		if err != nil {
			return notifRow{}, false
		}
		return n, true
	}

	n2201, ok := queryNotif(2201)
	require.True(t, ok, "expected tastemaker notif for user 2201")
	assert.Equal(t, []int64{2201}, n2201.UserIDs)
	assert.Equal(t, "220000", n2201.Specifier)

	var data2201 map[string]any
	require.NoError(t, json.Unmarshal(n2201.Data, &data2201))
	assert.Equal(t, "repost", data2201["action"])
	assert.Equal(t, "track", data2201["tastemaker_item_type"])
	assert.EqualValues(t, 220000, data2201["tastemaker_item_id"])
	assert.EqualValues(t, 2200, data2201["tastemaker_item_owner_id"])
	assert.EqualValues(t, 2201, data2201["tastemaker_user_id"])

	n2202, ok := queryNotif(2202)
	require.True(t, ok, "expected tastemaker notif for user 2202")
	var data2202 map[string]any
	require.NoError(t, json.Unmarshal(n2202.Data, &data2202))
	assert.Equal(t, "repost", data2202["action"], "user has both repost and save — repost wins")
}
