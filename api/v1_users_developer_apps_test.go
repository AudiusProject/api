package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestV1UsersDeveloperApps(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "user1"},
		},
		"developer_apps": []map[string]any{
			{
				"address":     "app_address_1",
				"user_id":     1,
				"name":        "app_name_1",
				"description": "app_description_1",
				"is_current":  true,
				"is_delete":   false,
			},
			{
				"address":     "app_address_2",
				"user_id":     1,
				"name":        "app_name_2",
				"description": "app_description_2",
				"image_url":   "app_image_url_2",
				"is_current":  true,
				"is_delete":   false,
			},
			{
				"address": "app_address_3",
				"user_id": 1, "name": "app_name_3",
				"description": "app_description_3",
				"is_current":  false,
				"is_delete":   false,
			},
			{
				"address":     "app_address_4",
				"user_id":     1,
				"name":        "app_name_4",
				"description": "app_description_4",
				"is_current":  true,
				"is_delete":   true,
			},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	{
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/developer-apps")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":             2,
			"data.0.address":     "app_address_1",
			"data.0.user_id":     trashid.MustEncodeHashID(1),
			"data.0.name":        "app_name_1",
			"data.0.description": "app_description_1",
			"data.1.address":     "app_address_2",
			"data.1.user_id":     trashid.MustEncodeHashID(1),
			"data.1.name":        "app_name_2",
			"data.1.description": "app_description_2",
		})
		// redirect_uris must be an empty array (not null) when no URIs are registered
		assert.True(t, gjson.GetBytes(body, "data.0.redirect_uris").IsArray())
		assert.Equal(t, 0, len(gjson.GetBytes(body, "data.0.redirect_uris").Array()))
		assert.True(t, gjson.GetBytes(body, "data.1.redirect_uris").IsArray())
		assert.Equal(t, 0, len(gjson.GetBytes(body, "data.1.redirect_uris").Array()))
	}

	{
		status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(2)+"/developer-apps")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#": 0,
		})
	}
}

func TestV1UsersDeveloperAppsIncludeMetrics(t *testing.T) {
	app := emptyTestApp(t)
	// Use UTC so api_metrics row dates align with PostgreSQL CURRENT_DATE in tests.
	now := time.Now().UTC()
	// AddDate(0, -1, 0) from e.g. Mar 30 normalizes to Mar 2 (same month), which would
	// incorrectly include prior-month metrics in month-to-date. Use last day of prev month.
	firstOfMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	lastOfPrevMonth := firstOfMonth.AddDate(0, 0, -1)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "user1"},
		},
		"developer_apps": []map[string]any{
			{
				"address":     "app_address_1",
				"user_id":     1,
				"name":        "app_name_1",
				"description": "app_description_1",
				"is_current":  true,
				"is_delete":   false,
			},
		},
		"oauth_redirect_uris": []map[string]any{
			{"client_id": "app_address_1", "redirect_uri": "https://example.com/callback-a"},
			{"client_id": "app_address_1", "redirect_uri": "https://example.com/callback-b"},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)
	_, err := app.pool.Exec(t.Context(), `
		INSERT INTO api_metrics_apps (date, api_key, app_name, request_count)
		VALUES ($1, $2, $3, $4), ($5, $6, $7, $8)
	`, now, "app_address_1", "app_name_1", 7, lastOfPrevMonth, "app_address_1", "app_name_1", 3)
	assert.NoError(t, err)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/developer-apps?include=metrics")
	assert.Equal(t, 200, status, string(body))
	jsonAssert(t, body, map[string]any{
		"data.#":                        1,
		"data.0.address":                "app_address_1",
		"data.0.user_id":                trashid.MustEncodeHashID(1),
		"data.0.request_count":          float64(7),
		"data.0.request_count_all_time": float64(10),
		"data.0.is_legacy":              true,
		"data.0.redirect_uris.#":        2,
		"data.0.redirect_uris.0":        "https://example.com/callback-a",
		"data.0.redirect_uris.1":        "https://example.com/callback-b",
		"data.0.api_access_keys.#":      0,
	})
}

// TestV1UsersDeveloperAppsRedirectUrisOrdering verifies that redirect_uris are returned
// in deterministic order (by oauth_redirect_uris.id) rather than alphabetically,
// and that apps without registered URIs return an empty array rather than null.
func TestV1UsersDeveloperAppsRedirectUrisOrdering(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "handle": "user1"},
		},
		"developer_apps": []map[string]any{
			{
				"address":    "app_address_1",
				"user_id":    1,
				"name":       "app_name_1",
				"is_current": true,
				"is_delete":  false,
				"created_at": time.Now(),
			},
			{
				"address":    "app_address_2",
				"user_id":    1,
				"name":       "app_name_2",
				"is_current": true,
				"is_delete":  false,
				"created_at": time.Now().Add(-time.Second),
			},
		},
		// Insert z before a: if ordering were alphabetical, a would come first.
		// With id ordering, z comes first because it was inserted with a lower id.
		"oauth_redirect_uris": []map[string]any{
			{"client_id": "app_address_1", "redirect_uri": "https://z.example.com/callback"},
			{"client_id": "app_address_1", "redirect_uri": "https://a.example.com/callback"},
		},
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/developer-apps")
	assert.Equal(t, 200, status)
	// app_address_1 has 2 redirect URIs; z was inserted first (lower id) so it must come first
	jsonAssert(t, body, map[string]any{
		"data.#":                  2,
		"data.0.address":          "app_address_1",
		"data.0.redirect_uris.#":  2,
		"data.0.redirect_uris.0":  "https://z.example.com/callback",
		"data.0.redirect_uris.1":  "https://a.example.com/callback",
		"data.1.address":          "app_address_2",
	})
	// app_address_2 has no redirect URIs; must be an empty array, not null
	assert.True(t, gjson.GetBytes(body, "data.1.redirect_uris").IsArray())
	assert.Equal(t, 0, len(gjson.GetBytes(body, "data.1.redirect_uris").Array()))
}
