package challenges

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestTrending_IdempotentSameWeek seeds top-3 tracks for the current week
// in track_trending_scores and runs the trending tracks processor twice.
// The processor is gated to Fridays — this test skips itself on non-Fridays
// to avoid time-coupling. When it runs, both runs should produce the same
// 3 trending_results rows.
func TestTrending_IdempotentSameWeek(t *testing.T) {
	if time.Now().UTC().Weekday() != time.Friday {
		t.Skip("trending processor only runs on Fridays UTC")
	}
	pool := withChallengesDB(t)
	now := time.Now()
	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_tt", "number": 1}},
		"users": {
			{"user_id": 500, "wallet": "0x500"},
			{"user_id": 501, "wallet": "0x501"},
			{"user_id": 502, "wallet": "0x502"},
		},
		"tracks": {
			{"track_id": 5001, "owner_id": 500, "title": "A", "blocknumber": 1, "created_at": now},
			{"track_id": 5011, "owner_id": 501, "title": "B", "blocknumber": 1, "created_at": now},
			{"track_id": 5021, "owner_id": 502, "title": "C", "blocknumber": 1, "created_at": now},
		},
	})

	// Manually seed track_trending_scores (the trending job in api/parity-jobs
	// would normally populate this).
	for i, tid := range []int{5001, 5011, 5021} {
		_, err := pool.Exec(context.Background(), `
			INSERT INTO track_trending_scores (track_id, type, version, time_range, score, created_at)
			VALUES ($1, 'TRACKS', 'pnagD', 'week', $2, now())
		`, tid, float64(100-i))
		require.NoError(t, err)
	}

	runProcessor(t, pool, NewTrendingTrackProcessor())

	// Three rows in trending_results, ranked.
	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM trending_results WHERE type = 'TRACKS' AND version = 'pnagD'").Scan(&count))
	assert.Equal(t, 3, count)

	// User_challenges: one row per rank with amount = 1000 (top-5).
	for _, userID := range []int{500, 501, 502} {
		weekDate := time.Date(time.Now().UTC().Year(), time.Now().UTC().Month(), time.Now().UTC().Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
		var rank int32 = 1
		// We don't know which rank each user got; iterate.
		for r := int32(1); r <= 3; r++ {
			specifier := fmt.Sprintf("%s:%d", weekDate, r)
			ucRow, ok := queryUserChallenge(t, pool, "tt", specifier)
			if ok && ucRow.UserID == int64(userID) {
				rank = r
				assert.Equal(t, int32(1000), ucRow.Amount, "rank %d should pay 1000", rank)
				assert.True(t, ucRow.IsComplete)
				break
			}
		}
	}

	// Second run is a no-op (already paid this week).
	runProcessor(t, pool, NewTrendingTrackProcessor())
	var count2 int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM trending_results WHERE type = 'TRACKS' AND version = 'pnagD'").Scan(&count2))
	assert.Equal(t, 3, count2, "second run on same Friday should not duplicate")
}

func TestTrending_SkipsNonFriday(t *testing.T) {
	if time.Now().UTC().Weekday() == time.Friday {
		t.Skip("test only meaningful on non-Fridays")
	}
	pool := withChallengesDB(t)
	runProcessor(t, pool, NewTrendingTrackProcessor())

	var count int
	require.NoError(t, pool.QueryRow(context.Background(),
		"SELECT COUNT(*) FROM trending_results").Scan(&count))
	assert.Equal(t, 0, count, "no rows written on non-Friday")
}

// TestTrending_EmitsNotification — handle_trending trigger fans out a
// `trending` notification when a user_challenges row is minted for 'tt'.
// Skips on non-Fridays since the underlying processor is Friday-gated.
func TestTrending_EmitsNotification(t *testing.T) {
	if time.Now().UTC().Weekday() != time.Friday {
		t.Skip("trending processor only runs on Fridays UTC")
	}
	pool := withChallengesDB(t)
	ctx := context.Background()
	now := time.Now()

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_ttn", "number": 1}},
		"users": {
			{"user_id": 600, "wallet": "0x600"},
			{"user_id": 601, "wallet": "0x601"},
		},
		"tracks": {
			{"track_id": 6001, "owner_id": 600, "title": "Hit1", "blocknumber": 1, "created_at": now},
			{"track_id": 6011, "owner_id": 601, "title": "Hit2", "blocknumber": 1, "created_at": now},
		},
	})

	for i, tid := range []int{6001, 6011} {
		_, err := pool.Exec(ctx, `
			INSERT INTO track_trending_scores (track_id, type, version, time_range, score, created_at)
			VALUES ($1, 'TRACKS', 'pnagD', 'week', $2, now())
		`, tid, float64(100-i))
		require.NoError(t, err)
	}

	runProcessor(t, pool, NewTrendingTrackProcessor())

	weekDate := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	// Find the rank-1 row.
	var ownerID int64
	var specifier string
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT user_id, specifier FROM user_challenges
		WHERE challenge_id = 'tt' AND specifier = $1
	`, fmt.Sprintf("%s:1", weekDate)).Scan(&ownerID, &specifier))

	// Find the corresponding notification.
	var nUserIDs []int64
	var nSpecifier, nGroupID string
	var nData []byte
	err := pool.QueryRow(ctx, `
		SELECT user_ids, specifier, group_id, data
		FROM notification
		WHERE type = 'trending' AND user_ids = ARRAY[$1::int]
	`, ownerID).Scan(&nUserIDs, &nSpecifier, &nGroupID, &nData)
	require.NoError(t, err, "expected trending notif for owner %d", ownerID)

	var data map[string]any
	require.NoError(t, json.Unmarshal(nData, &data))
	assert.Equal(t, "week", data["time_range"])
	assert.Equal(t, "all", data["genre"])
	assert.EqualValues(t, 1, data["rank"])
	assert.Contains(t, data, "track_id")
	assert.Contains(t, nGroupID, ":rank:1:track_id:")
	assert.Contains(t, nGroupID, "trending:time_range:week:genre:all")
}

// TestTrendingPlaylist_EmitsNotification — handle_trending fans out a
// `trending_playlist` notification with playlist_id (not track_id) in
// the data payload and group_id.
func TestTrendingPlaylist_EmitsNotification(t *testing.T) {
	if time.Now().UTC().Weekday() != time.Friday {
		t.Skip("trending processor only runs on Fridays UTC")
	}
	pool := withChallengesDB(t)
	ctx := context.Background()
	now := time.Now()

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_tpn", "number": 1}},
		"users":  {{"user_id": 700, "wallet": "0x700"}},
		"playlists": {{
			"playlist_id":       7001,
			"playlist_owner_id": 700,
			"playlist_name":     "P",
			"blocknumber":       1,
			"created_at":        now,
		}},
	})

	_, err := pool.Exec(ctx, `
		INSERT INTO playlist_trending_scores (playlist_id, type, version, time_range, score, created_at)
		VALUES (7001, 'PLAYLISTS', 'pnagD', 'week', 999.0, now())
	`)
	require.NoError(t, err)

	runProcessor(t, pool, NewTrendingPlaylistProcessor())

	var nGroupID string
	var nData []byte
	err = pool.QueryRow(ctx, `
		SELECT group_id, data
		FROM notification
		WHERE type = 'trending_playlist' AND user_ids = ARRAY[700]
	`).Scan(&nGroupID, &nData)
	require.NoError(t, err, "expected trending_playlist notif")

	var data map[string]any
	require.NoError(t, json.Unmarshal(nData, &data))
	assert.EqualValues(t, 7001, data["playlist_id"])
	assert.NotContains(t, data, "track_id")
	assert.Contains(t, nGroupID, ":playlist_id:7001:")
	assert.Contains(t, nGroupID, "trending_playlist:time_range:week:genre:all")
}
