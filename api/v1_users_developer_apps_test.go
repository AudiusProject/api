package api

import (
	"testing"

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
