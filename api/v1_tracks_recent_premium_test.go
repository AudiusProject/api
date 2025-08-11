package api

import (
	"fmt"
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1TracksRecentPremium(t *testing.T) {
	app := emptyTestApp(t)

	users := []map[string]any{}
	for i := range 5 {
		userId := i + 1
		users = append(users, map[string]any{
			"user_id": userId,
			"handle":  fmt.Sprintf("user%d", userId),
		})
	}

	makeUsdcConditions := func(userId int) string {
		return fmt.Sprintf("{\"usdc_purchase\": {\"price\": 100, \"splits\": [{\"user_id\": %d, \"percentage\": 100}]}}", userId)
	}

	fixtures := database.FixtureMap{
		"users": users,
		"tracks": []map[string]any{
			// Two tracks from user 1 at the top of the list, should only get one back
			{
				"track_id":          1,
				"owner_id":          1,
				"created_at":        time.Now().Add(-time.Duration(1) * time.Hour),
				"stream_conditions": makeUsdcConditions(1),
			},
			{
				"track_id":            2,
				"owner_id":            1,
				"created_at":          time.Now().Add(-time.Duration(2) * time.Hour),
				"download_conditions": makeUsdcConditions(2),
			},
			{
				"track_id":          3,
				"owner_id":          2,
				"created_at":        time.Now().Add(-time.Duration(3) * time.Hour),
				"stream_conditions": makeUsdcConditions(2),
			},
			{
				"track_id":            4,
				"owner_id":            3,
				"created_at":          time.Now().Add(-time.Duration(4) * time.Hour),
				"download_conditions": makeUsdcConditions(3),
			},
			{
				"track_id":          5,
				"owner_id":          4,
				"created_at":        time.Now().Add(-time.Duration(5) * time.Hour),
				"stream_conditions": makeUsdcConditions(4),
			},
			{
				"track_id":          6,
				"owner_id":          5,
				"created_at":        time.Now().Add(-time.Duration(6) * time.Hour),
				"stream_conditions": makeUsdcConditions(5),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	{
		status, body := testGet(t, app, "/v1/full/tracks/recent-premium")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":         5,
			"data.0.id":      trashid.MustEncodeHashID(1),
			"data.0.user.id": trashid.MustEncodeHashID(1),
			"data.1.id":      trashid.MustEncodeHashID(3),
			"data.1.user.id": trashid.MustEncodeHashID(2),
			"data.2.id":      trashid.MustEncodeHashID(4),
			"data.2.user.id": trashid.MustEncodeHashID(3),
			"data.3.id":      trashid.MustEncodeHashID(5),
			"data.3.user.id": trashid.MustEncodeHashID(4),
			"data.4.id":      trashid.MustEncodeHashID(6),
			"data.4.user.id": trashid.MustEncodeHashID(5),
		})
	}

	{
		status, body := testGet(t, app, "/v1/tracks/recent-premium")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":    5,
			"data.0.id": trashid.MustEncodeHashID(1),
			"data.1.id": trashid.MustEncodeHashID(3),
			"data.2.id": trashid.MustEncodeHashID(4),
			"data.3.id": trashid.MustEncodeHashID(5),
			"data.4.id": trashid.MustEncodeHashID(6),
		})
	}

	{
		status, body := testGet(t, app, "/v1/tracks/recent-premium?limit=1&offset=1")
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#":    1,
			"data.0.id": trashid.MustEncodeHashID(3),
		})
	}
}

func TestV1TracksRecentPremiumInvalidParams(t *testing.T) {
	app := emptyTestApp(t)

	{
		status, _ := testGet(t, app, "/v1/tracks/recent-premium?limit=-1")
		assert.Equal(t, 400, status)
	}

	{
		status, _ := testGet(t, app, "/v1/tracks/recent-premium?offset=-1")
		assert.Equal(t, 400, status)
	}

	{
		status, _ := testGet(t, app, "/v1/tracks/recent-premium?limit=101")
		assert.Equal(t, 400, status)
	}

	{
		status, _ := testGet(t, app, "/v1/tracks/recent-premium?limit=invalid")
		assert.Equal(t, 400, status)
	}

	{
		status, _ := testGet(t, app, "/v1/tracks/recent-premium?offset=invalid")
		assert.Equal(t, 400, status)
	}
}
