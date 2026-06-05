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

// TestTrending_UndergroundFiltersMainstream is the regression test for the
// underground winners filter. It seeds a realistic mix — a 20+ track mainstream
// top list owned by high-follower artists, plus a few genuinely niche tracks —
// and asserts the underground processor pays out ONLY the niche tracks. Before
// the fix the underground path read raw UNDERGROUND_TRACKS scores (computed
// identically to TRACKS, with no eligibility filter), so its winners were
// effectively the same as regular trending. Friday-gated like the others.
func TestTrending_UndergroundFiltersMainstream(t *testing.T) {
	if time.Now().UTC().Weekday() != time.Friday {
		t.Skip("trending processor only runs on Fridays UTC")
	}
	pool := withChallengesDB(t)
	ctx := context.Background()
	now := time.Now()

	userRows := []map[string]any{}
	trackRows := []map[string]any{}

	// 22 mainstream tracks (ids 7000..7021) by high-follower artists
	// (ids 700..721), descending scores 1000..979. The top 20 form the
	// global trending list; all 22 owners are too popular to be underground.
	for i := 0; i < 22; i++ {
		uid := 700 + i
		tid := 7000 + i
		userRows = append(userRows, map[string]any{"user_id": uid, "wallet": fmt.Sprintf("0x%d", uid)})
		trackRows = append(trackRows, map[string]any{"track_id": tid, "owner_id": uid, "title": fmt.Sprintf("Main%d", i), "blocknumber": 1, "created_at": now})
	}
	// 3 genuinely underground tracks (7030..7032) by niche artists
	// (730..732): few followers, few follows, scores well below the top 20.
	for i := 0; i < 3; i++ {
		uid := 730 + i
		tid := 7030 + i
		userRows = append(userRows, map[string]any{"user_id": uid, "wallet": fmt.Sprintf("0x%d", uid)})
		trackRows = append(trackRows, map[string]any{"track_id": tid, "owner_id": uid, "title": fmt.Sprintf("Under%d", i), "blocknumber": 1, "created_at": now})
	}
	// Two artists that should still be excluded even though their score is
	// below the top 20: 740 has too many followers, 741 follows too many.
	userRows = append(userRows,
		map[string]any{"user_id": 740, "wallet": "0x740"},
		map[string]any{"user_id": 741, "wallet": "0x741"},
	)
	trackRows = append(trackRows,
		map[string]any{"track_id": 7040, "owner_id": 740, "title": "PopularLowScore", "blocknumber": 1, "created_at": now},
		map[string]any{"track_id": 7041, "owner_id": 741, "title": "FollowsEveryone", "blocknumber": 1, "created_at": now},
	)

	database.Seed(pool, database.FixtureMap{
		"blocks": {{"blockhash": "blk_ug", "number": 1}},
		"users":  userRows,
		"tracks": trackRows,
	})

	// Owner follower/following counts. aggregate_user rows already exist via
	// the users-insert trigger (default 0), so upsert the real values.
	setCounts := func(uid, followers, following int) {
		_, err := pool.Exec(ctx, `
			INSERT INTO aggregate_user (user_id, follower_count, following_count)
			VALUES ($1, $2, $3)
			ON CONFLICT (user_id) DO UPDATE
			SET follower_count = EXCLUDED.follower_count,
			    following_count = EXCLUDED.following_count
		`, uid, followers, following)
		require.NoError(t, err)
	}
	for i := 0; i < 22; i++ {
		setCounts(700+i, 5000, 50) // mainstream: too many followers
	}
	for i := 0; i < 3; i++ {
		setCounts(730+i, 100, 100) // underground: eligible
	}
	setCounts(740, 5000, 50)   // excluded by follower cap
	setCounts(741, 100, 5000)  // excluded by following cap

	// Scores: mainstream 1000..979, underground 500/499/498, the two
	// excluded low-score artists at 480/470 (below the top 20 cutoff but
	// still filtered out by the follower/following caps).
	insertScore := func(tid int, score float64) {
		_, err := pool.Exec(ctx, `
			INSERT INTO track_trending_scores (track_id, type, version, time_range, score, created_at)
			VALUES ($1, 'TRACKS', 'pnagD', 'week', $2, now())
		`, tid, score)
		require.NoError(t, err)
	}
	for i := 0; i < 22; i++ {
		insertScore(7000+i, float64(1000-i))
	}
	insertScore(7030, 500)
	insertScore(7031, 499)
	insertScore(7032, 498)
	insertScore(7040, 480)
	insertScore(7041, 470)

	runProcessor(t, pool, NewTrendingUndergroundProcessor())

	// trending_results for UNDERGROUND_TRACKS should be exactly the three
	// niche tracks, ranked by score desc — no mainstream or capped artists.
	rows, err := pool.Query(ctx, `
		SELECT id, rank FROM trending_results
		WHERE type = 'UNDERGROUND_TRACKS' AND version = 'pnagD'
		ORDER BY rank
	`)
	require.NoError(t, err)
	var gotIDs []string
	var gotRanks []int32
	for rows.Next() {
		var id string
		var rank int32
		require.NoError(t, rows.Scan(&id, &rank))
		gotIDs = append(gotIDs, id)
		gotRanks = append(gotRanks, rank)
	}
	rows.Close()
	require.NoError(t, rows.Err())

	assert.Equal(t, []string{"7030", "7031", "7032"}, gotIDs,
		"underground winners must be the niche tracks only, in score order")
	assert.Equal(t, []int32{1, 2, 3}, gotRanks)

	// And a user_challenge should be minted for each underground owner.
	weekDate := time.Date(now.UTC().Year(), now.UTC().Month(), now.UTC().Day(), 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	for rank, owner := range map[int32]int64{1: 730, 2: 731, 3: 732} {
		uc, ok := queryUserChallenge(t, pool, "tut", fmt.Sprintf("%s:%d", weekDate, rank))
		require.True(t, ok, "expected tut challenge for rank %d", rank)
		assert.Equal(t, owner, uc.UserID)
		assert.Equal(t, int32(1000), uc.Amount)
	}
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
