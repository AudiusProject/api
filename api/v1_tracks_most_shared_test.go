package api

import (
	"fmt"
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1TracksMostShared(t *testing.T) {
	app := emptyTestApp(t)

	users := []map[string]any{}
	for i := range 5 {
		userId := i + 1
		users = append(users, map[string]any{
			"user_id": userId,
			"handle":  fmt.Sprintf("user%d", userId),
		})
	}

	tracks := []map[string]any{}
	for i := range 5 {
		trackId := i + 1
		tracks = append(tracks, map[string]any{
			"track_id": trackId,
			"owner_id": trackId % 5,
		})
	}

	fixtures := database.FixtureMap{
		"users":  users,
		"tracks": tracks,
		"shares": []map[string]any{
			{
				"share_item_id": 1,
				"user_id":       1,
				"share_type":    "track",
				"created_at":    time.Now(),
			},
			{
				"share_item_id": 1,
				"user_id":       2,
				"share_type":    "track",
				"created_at":    time.Now(),
			},
			{
				"share_item_id": 2,
				"user_id":       1,
				"share_type":    "track",
				"created_at":    time.Now(),
			},
			{
				"share_item_id": 2,
				"user_id":       2,
				"share_type":    "track",
				"created_at":    time.Now().Add(-time.Duration(24*7*2) * time.Hour), // 2 weeks ago
			},
			{
				"share_item_id": 2,
				"user_id":       3,
				"share_type":    "track",
				"created_at":    time.Now().Add(-time.Duration(24*7*2) * time.Hour), // 2 weeks ago
			},
			{
				"share_item_id": 3,
				"user_id":       1,
				"share_type":    "track",
				"created_at":    time.Now().Add(-time.Duration(24*7*5) * time.Hour), // 5 weeks ago
			},
			{
				"share_item_id": 3,
				"user_id":       2,
				"share_type":    "track",
				"created_at":    time.Now().Add(-time.Duration(24*7*5) * time.Hour), // 5 weeks ago
			},
			{
				"share_item_id": 3,
				"user_id":       3,
				"share_type":    "track",
				"created_at":    time.Now().Add(-time.Duration(24*7*5) * time.Hour), // 5 weeks ago
			},
			{
				"share_item_id": 3,
				"user_id":       4,
				"share_type":    "track",
				"created_at":    time.Now().Add(-time.Duration(24*7*5) * time.Hour), // 5 weeks ago
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// Default case (by week), track 1 has most shares, track 3 has none in the past week
	{
		status, body := testGet(t, app, "/v1/tracks/most-shared")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":    2,
			"data.0.id": trashid.MustEncodeHashID(1),
			"data.1.id": trashid.MustEncodeHashID(2),
		})
	}

	// By month, track 2 has most shares
	{
		status, body := testGet(t, app, "/v1/tracks/most-shared?time_range=month")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":    2,
			"data.0.id": trashid.MustEncodeHashID(2),
			"data.1.id": trashid.MustEncodeHashID(1),
		})
	}

	// By all time, track 3 has most shares
	{
		status, body := testGet(t, app, "/v1/tracks/most-shared?time_range=allTime")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":    3,
			"data.0.id": trashid.MustEncodeHashID(3),
			"data.1.id": trashid.MustEncodeHashID(2),
			"data.2.id": trashid.MustEncodeHashID(1),
		})
	}
}
