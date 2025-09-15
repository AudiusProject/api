package api

import (
	"fmt"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestTrackSubsequent(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"tracks": {
			{
				"track_id":   1,
				"owner_id":   1,
				"created_at": parseTimeWithLayout(t, "2025-01-01T00:00:00Z", time.RFC3339),
			},
			{
				"track_id":   4,
				"owner_id":   1,
				"created_at": parseTimeWithLayout(t, "2025-01-01T00:01:00Z", time.RFC3339),
			},
			{
				"track_id":   3,
				"owner_id":   1,
				"created_at": parseTimeWithLayout(t, "2025-01-01T00:01:00Z", time.RFC3339),
			},
			{
				"track_id":   5,
				"owner_id":   1,
				"created_at": parseTimeWithLayout(t, "2025-01-01T00:01:00Z", time.RFC3339),
			},
			{
				"track_id":   2,
				"owner_id":   1,
				"created_at": parseTimeWithLayout(t, "2025-01-01T00:02:00Z", time.RFC3339),
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	{
		status, body := testGet(t, app, fmt.Sprintf("/v1/tracks/%s/subsequent?limit=5", trashid.MustEncodeHashID(1)))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#": 4,
			// Deterministically sorted by track_id if same create time
			"data.0": trashid.MustEncodeHashID(3),
			"data.1": trashid.MustEncodeHashID(4),
			"data.2": trashid.MustEncodeHashID(5),
			"data.3": trashid.MustEncodeHashID(2),
		})
	}

	{
		status, body := testGet(t, app, fmt.Sprintf("/v1/tracks/%s/subsequent?limit=5", trashid.MustEncodeHashID(4)))
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#": 2,
			"data.0": trashid.MustEncodeHashID(5),
			"data.1": trashid.MustEncodeHashID(2),
		})
	}
}
