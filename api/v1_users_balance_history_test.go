package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestUserBalanceHistoryHourly(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)
	twoHoursAgo := now.Add(-2 * time.Hour)

	// Truncate to hour for consistent testing
	nowHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())
	hourAgoHour := time.Date(hourAgo.Year(), hourAgo.Month(), hourAgo.Day(), hourAgo.Hour(), 0, 0, 0, hourAgo.Location())
	twoHoursAgoHour := time.Date(twoHoursAgo.Year(), twoHoursAgo.Month(), twoHoursAgo.Day(), twoHoursAgo.Hour(), 0, 0, 0, twoHoursAgo.Location())

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id": 1,
				"wallet":  "0x1234567890123456789012345678901234567890",
			},
		},
		"artist_coins": {
			{
				"ticker":   "COIN1",
				"decimals": 6,
				"user_id":  1,
				"mint":     "mint1",
			},
			{
				"ticker":   "COIN2",
				"decimals": 6,
				"user_id":  1,
				"mint":     "mint2",
			},
		},
		"user_balance_history": {
			// 2 hours ago: user had 100 COIN1 ($50) + 200 COIN2 ($40) = $90 total
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   twoHoursAgoHour,
				"balance":     100000000, // 100 tokens
				"balance_usd": 50.0,
			},
			{
				"user_id":     1,
				"mint":        "mint2",
				"timestamp":   twoHoursAgoHour,
				"balance":     200000000, // 200 tokens
				"balance_usd": 40.0,
			},
			// 1 hour ago: user had 150 COIN1 ($75) + 200 COIN2 ($40) = $115 total
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   hourAgoHour,
				"balance":     150000000, // 150 tokens
				"balance_usd": 75.0,
			},
			{
				"user_id":     1,
				"mint":        "mint2",
				"timestamp":   hourAgoHour,
				"balance":     200000000, // 200 tokens
				"balance_usd": 40.0,
			},
			// Now: user has 200 COIN1 ($100) + 300 COIN2 ($60) = $160 total
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   nowHour,
				"balance":     200000000, // 200 tokens
				"balance_usd": 100.0,
			},
			{
				"user_id":     1,
				"mint":        "mint2",
				"timestamp":   nowHour,
				"balance":     300000000, // 300 tokens
				"balance_usd": 60.0,
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	// Test default (last 7 days) - should return hourly data with summed balances
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#": 3, // 3 distinct hourly timestamps
		// Most recent should be first when ordered ASC by timestamp
		// Actually, ORDER BY timestamp ASC, so oldest first
		"data.0.balance_usd": 90.0,  // 2 hours ago: 50 + 40
		"data.1.balance_usd": 115.0, // 1 hour ago: 75 + 40
		"data.2.balance_usd": 160.0, // now: 100 + 60
	})
}

func TestUserBalanceHistoryDaily(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()

	// Create data for 10 days ago and 5 days ago
	tenDaysAgo := now.Add(-10 * 24 * time.Hour)
	fiveDaysAgo := now.Add(-5 * 24 * time.Hour)

	// Truncate to hour
	tenDaysAgoHour := time.Date(tenDaysAgo.Year(), tenDaysAgo.Month(), tenDaysAgo.Day(), 10, 0, 0, 0, tenDaysAgo.Location())
	fiveDaysAgoHour := time.Date(fiveDaysAgo.Year(), fiveDaysAgo.Month(), fiveDaysAgo.Day(), 14, 0, 0, 0, fiveDaysAgo.Location())

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
		"user_balance_history": {
			// 10 days ago at 10:00
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   tenDaysAgoHour,
				"balance":     100000000,
				"balance_usd": 50.0,
			},
			// 5 days ago at 14:00
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   fiveDaysAgoHour,
				"balance":     200000000,
				"balance_usd": 100.0,
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	// Query for 15 days with daily granularity
	startTime := now.Add(-15 * 24 * time.Hour).Format(time.RFC3339)
	endTime := now.Format(time.RFC3339)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history?start_time="+startTime+"&end_time="+endTime+"&granularity=daily")
	assert.Equal(t, 200, status)

	// Should return 2 data points (different days)
	jsonAssert(t, body, map[string]any{
		"data.#": 2,
		// Verify data exists
		"data.0.balance_usd": 50.0,
		"data.1.balance_usd": 100.0,
	})
}

func TestUserBalanceHistoryMultipleMintsSummed(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()
	nowHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
			{"ticker": "COIN2", "decimals": 6, "user_id": 1, "mint": "mint2"},
			{"ticker": "COIN3", "decimals": 8, "user_id": 1, "mint": "mint3"},
		},
		"user_balance_history": {
			// All at same timestamp - should be summed
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   nowHour,
				"balance":     100000000,
				"balance_usd": 100.0,
			},
			{
				"user_id":     1,
				"mint":        "mint2",
				"timestamp":   nowHour,
				"balance":     200000000,
				"balance_usd": 200.0,
			},
			{
				"user_id":     1,
				"mint":        "mint3",
				"timestamp":   nowHour,
				"balance":     300000000,
				"balance_usd": 75.0,
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history")
	assert.Equal(t, 200, status)

	// Should return 1 data point with summed balance_usd
	jsonAssert(t, body, map[string]any{
		"data.#":             1,
		"data.0.balance_usd": 375.0, // 100 + 200 + 75
	})
}

func TestUserBalanceHistoryEmpty(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
	}

	database.Seed(app.writePool, fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history")
	assert.Equal(t, 200, status)

	// Should return empty array
	jsonAssert(t, body, map[string]any{
		"data.#": 0,
	})
}

func TestUserBalanceHistoryInvalidUserId(t *testing.T) {
	app := emptyTestApp(t)

	// Invalid user ID format
	status, _ := testGet(t, app, "/v1/users/invalid/balance/history")
	assert.Equal(t, 400, status)
}

func TestUserBalanceHistoryInvalidTimeRange(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
	}

	database.Seed(app.writePool, fixtures)

	// end_time before start_time
	startTime := time.Now().Format(time.RFC3339)
	endTime := time.Now().Add(-24 * time.Hour).Format(time.RFC3339)

	status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history?start_time="+startTime+"&end_time="+endTime)
	assert.Equal(t, 400, status)
}

func TestUserBalanceHistoryTimeRangeFiltering(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()

	// Create timestamps at different times
	timestamps := []time.Time{
		now.Add(-10 * 24 * time.Hour), // 10 days ago
		now.Add(-5 * 24 * time.Hour),  // 5 days ago
		now.Add(-2 * 24 * time.Hour),  // 2 days ago
		now.Add(-1 * time.Hour),       // 1 hour ago
	}

	// Truncate to hour
	for i := range timestamps {
		t := timestamps[i]
		timestamps[i] = time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
	}

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
		"user_balance_history": {
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   timestamps[0],
				"balance":     100000000,
				"balance_usd": 10.0,
			},
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   timestamps[1],
				"balance":     100000000,
				"balance_usd": 20.0,
			},
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   timestamps[2],
				"balance":     100000000,
				"balance_usd": 30.0,
			},
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   timestamps[3],
				"balance":     100000000,
				"balance_usd": 40.0,
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	// Query for last 3 days only (should filter out 10 days and 5 days ago)
	startTime := now.Add(-3 * 24 * time.Hour).Format(time.RFC3339)
	endTime := now.Format(time.RFC3339)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history?start_time="+startTime+"&end_time="+endTime)
	assert.Equal(t, 200, status)

	// Should only return data from last 3 days (2 days ago and 1 hour ago)
	jsonAssert(t, body, map[string]any{
		"data.#":             2,
		"data.0.balance_usd": 30.0, // 2 days ago
		"data.1.balance_usd": 40.0, // 1 hour ago
	})
}

func TestUserBalanceHistoryMultipleUsers(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()
	nowHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
			{"user_id": 2, "wallet": "0x0987654321098765432109876543210987654321"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
		"user_balance_history": {
			// User 1 data
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   nowHour,
				"balance":     100000000,
				"balance_usd": 100.0,
			},
			// User 2 data (should not appear in user 1's results)
			{
				"user_id":     2,
				"mint":        "mint1",
				"timestamp":   nowHour,
				"balance":     200000000,
				"balance_usd": 200.0,
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	// Query user 1 - should only get user 1's data
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":             1,
		"data.0.balance_usd": 100.0, // Only user 1's balance
	})

	// Query user 2 - should only get user 2's data
	status, body = testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(2)+"/balance/history")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":             1,
		"data.0.balance_usd": 200.0, // Only user 2's balance
	})
}

func TestUserBalanceHistoryGranularityParameter(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()

	// Create hourly data for 3 days
	var timestamps []time.Time
	for day := 0; day < 3; day++ {
		for hour := 0; hour < 24; hour += 6 { // 4 data points per day (00:00, 06:00, 12:00, 18:00)
			ts := now.Add(time.Duration(-day*24-hour) * time.Hour)
			tsHour := time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), 0, 0, 0, ts.Location())
			timestamps = append(timestamps, tsHour)
		}
	}

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
	}

	// Add balance history entries with different values per hour
	var balanceHistory []map[string]any
	balanceValue := 10.0
	for _, ts := range timestamps {
		balanceHistory = append(balanceHistory, map[string]any{
			"user_id":     1,
			"mint":        "mint1",
			"timestamp":   ts,
			"balance":     100000000,
			"balance_usd": balanceValue,
		})
		balanceValue += 1.0 // Increment to differentiate hours
	}
	fixtures["user_balance_history"] = balanceHistory

	database.Seed(app.writePool, fixtures)

	startTime := now.Add(-3 * 24 * time.Hour).Format(time.RFC3339)
	endTime := now.Format(time.RFC3339)

	// Test 1: hourly granularity (default) - should return all hourly data points
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history?start_time="+startTime+"&end_time="+endTime+"&granularity=hourly")
	assert.Equal(t, 200, status)

	// Should return hourly data points (4 per day × 3 days = 12 points)
	result := gjson.GetBytes(body, "data.#").Int()
	assert.True(t, result >= 10 && result <= 14, "Expected ~12 hourly points, got %d", result)

	// Test 2: daily granularity - should return daily aggregated data
	status, body = testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history?start_time="+startTime+"&end_time="+endTime+"&granularity=daily")
	assert.Equal(t, 200, status)

	// Should return daily data points (~3 points, one per day)
	result = gjson.GetBytes(body, "data.#").Int()
	assert.True(t, result >= 2 && result <= 4, "Expected ~3 daily points, got %d", result)

	// Test 3: default (no granularity param) - should default to hourly
	status, body = testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history?start_time="+startTime+"&end_time="+endTime)
	assert.Equal(t, 200, status)

	// Should return hourly data points (default behavior)
	result = gjson.GetBytes(body, "data.#").Int()
	assert.True(t, result >= 10 && result <= 14, "Expected ~12 hourly points (default), got %d", result)
}

func TestUserBalanceHistoryInvalidGranularity(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
	}

	database.Seed(app.writePool, fixtures)

	// Invalid granularity value
	status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history?granularity=weekly")
	assert.Equal(t, 400, status)
}

func TestUserBalanceHistoryDailyGranularityWithMultipleHours(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	// Create multiple hourly entries on the same day
	yesterdayHour1 := dayStart.Add(-24 * time.Hour).Add(2 * time.Hour)
	yesterdayHour2 := dayStart.Add(-24 * time.Hour).Add(10 * time.Hour)
	yesterdayHour3 := dayStart.Add(-24 * time.Hour).Add(18 * time.Hour)

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
		"user_balance_history": {
			// Multiple hourly entries on the same day - should be summed when using daily granularity
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   yesterdayHour1,
				"balance":     100000000,
				"balance_usd": 10.0,
			},
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   yesterdayHour2,
				"balance":     100000000,
				"balance_usd": 20.0,
			},
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   yesterdayHour3,
				"balance":     100000000,
				"balance_usd": 30.0,
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	startTime := dayStart.Add(-2 * 24 * time.Hour).Format(time.RFC3339)
	endTime := now.Format(time.RFC3339)

	// Test hourly granularity - should return 3 separate data points
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history?start_time="+startTime+"&end_time="+endTime+"&granularity=hourly")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#": 3, // 3 hourly points
	})

	// Test daily granularity - should return 1 data point with summed balance
	status, body = testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history?start_time="+startTime+"&end_time="+endTime+"&granularity=daily")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":             1,    // 1 daily point (all hours on same day grouped)
		"data.0.balance_usd": 60.0, // Sum: 10 + 20 + 30
	})
}

func TestUserBalanceHistoryOrdering(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()

	// Create 5 hourly data points in reverse chronological order
	var timestamps []time.Time
	for i := 0; i < 5; i++ {
		ts := now.Add(time.Duration(-i) * time.Hour)
		tsHour := time.Date(ts.Year(), ts.Month(), ts.Day(), ts.Hour(), 0, 0, 0, ts.Location())
		timestamps = append(timestamps, tsHour)
	}

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
	}

	// Add in reverse order to test ORDER BY
	var balanceHistoryOrdering []map[string]any
	for i, ts := range timestamps {
		balanceHistoryOrdering = append(balanceHistoryOrdering, map[string]any{
			"user_id":     1,
			"mint":        "mint1",
			"timestamp":   ts,
			"balance":     100000000,
			"balance_usd": float64(10 + i),
		})
	}
	fixtures["user_balance_history"] = balanceHistoryOrdering

	database.Seed(app.writePool, fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history")
	assert.Equal(t, 200, status)

	// Should be ordered ASC by timestamp (oldest first)
	jsonAssert(t, body, map[string]any{
		"data.#":             5,
		"data.0.balance_usd": 14.0, // 4 hours ago (oldest)
		"data.1.balance_usd": 13.0, // 3 hours ago
		"data.2.balance_usd": 12.0, // 2 hours ago
		"data.3.balance_usd": 11.0, // 1 hour ago
		"data.4.balance_usd": 10.0, // now (newest)
	})
}

func TestUserBalanceHistoryWithCustomTimeRange(t *testing.T) {
	app := emptyTestApp(t)

	// Fixed timestamps for predictable testing
	t1 := time.Date(2025, 1, 1, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2025, 1, 1, 11, 0, 0, 0, time.UTC)
	t3 := time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC)
	t4 := time.Date(2025, 1, 1, 13, 0, 0, 0, time.UTC)

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
		"user_balance_history": {
			{"user_id": 1, "mint": "mint1", "timestamp": t1, "balance": 100000000, "balance_usd": 10.0},
			{"user_id": 1, "mint": "mint1", "timestamp": t2, "balance": 100000000, "balance_usd": 20.0},
			{"user_id": 1, "mint": "mint1", "timestamp": t3, "balance": 100000000, "balance_usd": 30.0},
			{"user_id": 1, "mint": "mint1", "timestamp": t4, "balance": 100000000, "balance_usd": 40.0},
		},
	}

	database.Seed(app.writePool, fixtures)

	// Query for specific 2-hour window (11:00-13:00)
	startTime := t2.Format(time.RFC3339)
	endTime := t4.Format(time.RFC3339)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history?start_time="+startTime+"&end_time="+endTime)
	assert.Equal(t, 200, status)

	// Should return 3 points: 11:00, 12:00, 13:00
	jsonAssert(t, body, map[string]any{
		"data.#":             3,
		"data.0.balance_usd": 20.0, // 11:00
		"data.1.balance_usd": 30.0, // 12:00
		"data.2.balance_usd": 40.0, // 13:00
	})
}

func TestUserBalanceHistoryResponseFormat(t *testing.T) {
	app := emptyTestApp(t)

	now := time.Now()
	nowHour := time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, now.Location())

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": "mint1"},
		},
		"user_balance_history": {
			{
				"user_id":     1,
				"mint":        "mint1",
				"timestamp":   nowHour,
				"balance":     100000000,
				"balance_usd": 123.45,
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	var resp BalanceHistoryResponse
	status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/balance/history", &resp)
	assert.Equal(t, 200, status)

	// Verify structure
	assert.NotNil(t, resp.Data)
	assert.Equal(t, 1, len(resp.Data))
	assert.Greater(t, resp.Data[0].Timestamp, int64(0))
	assert.Equal(t, 123.45, resp.Data[0].BalanceUsd)
}
