package api

import (
	"fmt"
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersListenCountsMonthly(t *testing.T) {
	app := emptyTestApp(t)
	userID := 5001
	trackID := 6001
	track2ID := 6002
	now := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	nextMonth := now.AddDate(0, 1, 0)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id": userID,
				"handle":  "listenuser",
			},
		},
		"tracks": []map[string]any{
			{
				"track_id": trackID,
				"owner_id": userID,
				"title":    "Listen Track",
			},
			{
				"track_id": track2ID,
				"owner_id": userID,
				"title":    "Listen Track 2",
			},
		},
		"aggregate_monthly_plays": []map[string]any{
			{
				"play_item_id": trackID,
				"timestamp":    now,
				"count":        123,
			},
			{
				"play_item_id": track2ID,
				"timestamp":    now,
				"count":        5,
			},
			{
				"play_item_id": trackID,
				"timestamp":    nextMonth,
				"count":        456,
			},
		},
	}

	database.Seed(app.pool, fixtures)

	start := now.Format(time.RFC3339)
	end := nextMonth.AddDate(0, 1, 0).Format(time.RFC3339)
	url := fmt.Sprintf("/v1/users/%s/listen_counts_monthly?start_time=%s&end_time=%s",
		trashid.MustEncodeHashID(int(userID)),
		start,
		end,
	)

	status, body := testGet(t, app, url, userID)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data." + now.Format(time.RFC3339) + ".totalListens":           128,
		"data." + now.Format(time.RFC3339) + ".trackIds.0":             trackID,
		"data." + now.Format(time.RFC3339) + ".trackIds.1":             track2ID,
		"data." + now.Format(time.RFC3339) + ".listenCounts.0.trackId": trackID,
		"data." + now.Format(time.RFC3339) + ".listenCounts.0.listens": 123,
		"data." + now.Format(time.RFC3339) + ".listenCounts.1.trackId": track2ID,
		"data." + now.Format(time.RFC3339) + ".listenCounts.1.listens": 5,
		"data." + nextMonth.Format(time.RFC3339) + ".totalListens":     456,
	})
}
