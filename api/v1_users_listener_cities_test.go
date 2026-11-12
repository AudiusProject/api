package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestV1UsersListenerCities(t *testing.T) {
	artistWallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
	managerWallet := "0x4954d18926ba0ed9378938444731be4e622537b2"
	otherWallet := "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"
	app := emptyTestApp(t)

	today := time.Now().UTC().Truncate(24 * time.Hour)
	twoWeeksAgo := today.AddDate(0, 0, -14)
	twoMonthsAgo := today.AddDate(0, -2, 0)

	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": artistWallet, "handle": "artist", "handle_lc": "artist"},
			{"user_id": 2, "wallet": otherWallet, "handle": "other", "handle_lc": "other"},
			{"user_id": 3, "wallet": managerWallet, "handle": "manager", "handle_lc": "manager"},
		},
		"grants": {
			{
				"user_id":         1,
				"grantee_address": managerWallet,
				"is_current":      true,
				"is_approved":     true,
				"is_revoked":      false,
				"created_at":      time.Now(),
				"updated_at":      time.Now(),
			},
		},
		"tracks": {
			{"track_id": 100, "owner_id": 1, "title": "A"},
			{"track_id": 101, "owner_id": 1, "title": "B"},
			{"track_id": 200, "owner_id": 2, "title": "Someone else's"},
		},
		"aggregate_daily_city_plays": {
			// Atlanta: 3 this week across two tracks, 5 two months ago.
			{"play_item_id": 100, "day": today, "city": "Atlanta", "region": "Georgia", "country": "United States", "count": 2},
			{"play_item_id": 101, "day": today, "city": "Atlanta", "region": "Georgia", "country": "United States", "count": 1},
			{"play_item_id": 100, "day": twoMonthsAgo, "city": "Atlanta", "region": "Georgia", "country": "United States", "count": 5},
			// Berlin: 2 this week.
			{"play_item_id": 100, "day": today, "city": "Berlin", "region": "Land Berlin", "country": "Germany", "count": 2},
			// Lagos: 4 two weeks ago.
			{"play_item_id": 101, "day": twoWeeksAgo, "city": "Lagos", "region": "Lagos", "country": "Nigeria", "count": 4},
			// Another artist's track is not counted.
			{"play_item_id": 200, "day": today, "city": "Tokyo", "region": "Tokyo", "country": "Japan", "count": 50},
		},
	})

	var res struct {
		Data []ListenerCity
	}
	path := "/v1/users/" + trashid.MustEncodeHashID(1) + "/listener_cities"

	t.Run("week", func(t *testing.T) {
		status, _ := testGetWithWallet(t, app, path+"?time=week", artistWallet, &res)
		require.Equal(t, 200, status)
		assert.Equal(t, []ListenerCity{
			{City: "Atlanta", Region: "Georgia", Country: "United States", PlayCount: 3},
			{City: "Berlin", Region: "Land Berlin", Country: "Germany", PlayCount: 2},
		}, res.Data)
	})

	t.Run("month is the default", func(t *testing.T) {
		status, _ := testGetWithWallet(t, app, path, artistWallet, &res)
		require.Equal(t, 200, status)
		assert.Equal(t, []ListenerCity{
			{City: "Lagos", Region: "Lagos", Country: "Nigeria", PlayCount: 4},
			{City: "Atlanta", Region: "Georgia", Country: "United States", PlayCount: 3},
			{City: "Berlin", Region: "Land Berlin", Country: "Germany", PlayCount: 2},
		}, res.Data)
	})

	t.Run("all with limit", func(t *testing.T) {
		status, _ := testGetWithWallet(t, app, path+"?time=all&limit=2", artistWallet, &res)
		require.Equal(t, 200, status)
		assert.Equal(t, []ListenerCity{
			{City: "Atlanta", Region: "Georgia", Country: "United States", PlayCount: 8},
			{City: "Lagos", Region: "Lagos", Country: "Nigeria", PlayCount: 4},
		}, res.Data)
	})

	t.Run("manager can read", func(t *testing.T) {
		status, _ := testGetWithWallet(t, app, path+"?time=week", managerWallet, &res)
		require.Equal(t, 200, status)
		assert.Len(t, res.Data, 2)
	})

	t.Run("other users cannot", func(t *testing.T) {
		status, _ := testGetWithWallet(t, app, path, otherWallet)
		assert.Equal(t, 403, status)
	})

	t.Run("bad time", func(t *testing.T) {
		status, _ := testGetWithWallet(t, app, path+"?time=year", artistWallet)
		assert.Equal(t, 400, status)
	})
}
