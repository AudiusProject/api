package api

import (
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestUsersAuthorizedApps(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id": 1,
				"wallet":  "0x1234567890123456789012345678901234567890",
			},
			{
				"user_id": 2,
				"wallet":  "0x098765432109876543210987654321",
			},
		},
		"developer_apps": {
			{
				"address":     "app_address_1",
				"user_id":     1,
				"name":        "Test App 1",
				"description": "A test app",
				"image_url":   "https://example.com/image1.jpg",
				"is_current":  true,
				"is_delete":   false,
				"created_at":  time.Now().Add(-time.Hour),
			},
			{
				"address":     "app_address_2",
				"user_id":     2,
				"name":        "Test App 2",
				"description": "Another test app",
				"image_url":   "https://example.com/image2.jpg",
				"is_current":  true,
				"is_delete":   false,
				"created_at":  time.Now().Add(-30 * time.Minute),
			},
			{
				"address":     "app_address_3",
				"user_id":     3,
				"name":        "Deleted App",
				"description": "This app should be ignored",
				"image_url":   "https://example.com/image3.jpg",
				"is_current":  true,
				"is_delete":   true, // Should be filtered out
				"created_at":  time.Now(),
			},
		},
		"grants": {
			{
				"user_id":         1,
				"grantee_address": "app_address_1",
				"is_revoked":      false,
				"is_current":      true,
				"created_at":      time.Now().Add(-time.Hour),
				"updated_at":      time.Now().Add(-30 * time.Minute),
			},
			{
				"user_id":         1,
				"grantee_address": "app_address_2",
				"is_revoked":      false,
				"is_current":      true,
				"created_at":      time.Now().Add(-45 * time.Minute),
				"updated_at":      time.Now().Add(-15 * time.Minute),
			},
			{
				"user_id":         2,
				"grantee_address": "app_address_1",
				"is_revoked":      true, // Should be filtered out
				"is_current":      true,
				"created_at":      time.Now().Add(-time.Hour),
				"updated_at":      time.Now().Add(-30 * time.Minute),
			},
		},
	}

	database.Seed(app.pool, fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/authorized_apps")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":                 2,
		"data.0.address":         "app_address_1",
		"data.0.name":            "Test App 1",
		"data.0.description":     "A test app",
		"data.0.image_url":       "https://example.com/image1.jpg",
		"data.0.grantor_user_id": trashid.MustEncodeHashID(1),
		"data.1.address":         "app_address_2",
		"data.1.name":            "Test App 2",
		"data.1.description":     "Another test app",
		"data.1.image_url":       "https://example.com/image2.jpg",
		"data.1.grantor_user_id": trashid.MustEncodeHashID(1),
	})
}

func TestUsersAuthorizedAppsEmpty(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id": 1,
				"wallet":  "0x1234567890123456789012345678901234567890",
			},
		},
	}

	database.Seed(app.pool, fixtures)

	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/authorized_apps")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#": 0,
	})
}

func TestUsersAuthorizedAppsPagination(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id": 1,
				"wallet":  "0x1234567890123456789012345678901234567890",
			},
		},
		"developer_apps": {
			{
				"address":     "app_address_1",
				"user_id":     1,
				"name":        "Test App 1",
				"description": "A test app",
				"is_current":  true,
				"is_delete":   false,
				"created_at":  time.Now().Add(-time.Hour),
			},
			{
				"address":     "app_address_2",
				"user_id":     1,
				"name":        "Test App 2",
				"description": "Another test app",
				"is_current":  true,
				"is_delete":   false,
				"created_at":  time.Now().Add(-30 * time.Minute),
			},
		},
		"grants": {
			{
				"user_id":         1,
				"grantee_address": "app_address_1",
				"is_revoked":      false,
				"is_current":      true,
				"created_at":      time.Now().Add(-time.Hour),
				"updated_at":      time.Now().Add(-30 * time.Minute),
			},
			{
				"user_id":         1,
				"grantee_address": "app_address_2",
				"is_revoked":      false,
				"is_current":      true,
				"created_at":      time.Now().Add(-45 * time.Minute),
				"updated_at":      time.Now().Add(-15 * time.Minute),
			},
		},
	}

	database.Seed(app.pool, fixtures)

	// Test limit=1
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/authorized_apps?limit=1")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":         1,
		"data.0.address": "app_address_1",
	})

	// Test offset=1
	status, body = testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/authorized_apps?offset=1")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#":         1,
		"data.0.address": "app_address_2",
	})
}
