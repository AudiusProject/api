package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
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
	now := time.Now()

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
	`, now, "app_address_1", "app_name_1", 7, now.AddDate(0, -1, 0), "app_address_1", "app_name_1", 3)
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
