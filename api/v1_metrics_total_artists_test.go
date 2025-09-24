package api

import (
	"testing"

	"api.audius.co/database"
)

func TestMetricsTotalArtists_Empty(t *testing.T) {
	app := emptyTestApp(t)

	status, body := testGet(t, app, "/v1/metrics/total_artists")
	if status != 200 {
		t.Fatalf("expected 200, got %d: %s", status, string(body))
	}

	jsonAssert(t, body, map[string]any{
		"data.total": 0,
	})
}

func TestMetricsTotalArtists_WithFixtures(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"aggregate_user": {
			{"user_id": 1, "total_track_count": 0},
			{"user_id": 2, "total_track_count": 3},
			{"user_id": 3, "total_track_count": 10},
		},
		"users": {
			{"user_id": 1, "handle": "u1"},
			{"user_id": 2, "handle": "u2"},
			{"user_id": 3, "handle": "u3"},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/metrics/total_artists")
	if status != 200 {
		t.Fatalf("expected 200, got %d: %s", status, string(body))
	}

	jsonAssert(t, body, map[string]any{
		"data.total": 2,
	})
}
