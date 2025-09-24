package api

import (
	"testing"

	"api.audius.co/database"
)

func TestMetricsTotalPlays_Empty(t *testing.T) {
	app := emptyTestApp(t)

	status, body := testGet(t, app, "/v1/metrics/total_plays")
	if status != 200 {
		t.Fatalf("expected 200, got %d: %s", status, string(body))
	}

	jsonAssert(t, body, map[string]any{
		"data.total": 0,
	})
}

func TestMetricsTotalPlays_WithFixtures(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"aggregate_plays": {
			{"play_item_id": 1, "count": 100},
			{"play_item_id": 2, "count": 50},
			{"play_item_id": 3, "count": 30},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/metrics/total_plays")
	if status != 200 {
		t.Fatalf("expected 200, got %d: %s", status, string(body))
	}

	jsonAssert(t, body, map[string]any{
		"data.total": 180,
	})
}
