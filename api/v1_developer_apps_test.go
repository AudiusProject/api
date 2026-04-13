package api

import (
	"strings"
	"testing"

	"api.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

const testDeveloperAppAddress = "0x7d7b6b7a97d1deefe3a1ccc5a13c48e8f055e0b6"

func TestGetDeveloperAppsQueries(t *testing.T) {
	app := testAppWithFixtures(t)
	developerApps, err := app.queries.GetDeveloperApps(t.Context(), dbv1.GetDeveloperAppsParams{
		UserID: 1,
	})
	assert.NoError(t, err)
	assert.Len(t, developerApps, 1)
	assert.Equal(t, testDeveloperAppAddress, developerApps[0].Address)
	// redirect_uris must be an empty slice (not nil) when no URIs are registered
	assert.Equal(t, []string{}, developerApps[0].RedirectUris)
}

func TestGetDeveloperApp(t *testing.T) {
	app := testAppWithFixtures(t)

	var resp struct {
		Data dbv1.GetDeveloperAppsRow
	}
	status, body := testGet(t, app, "/v1/developer_apps/"+testDeveloperAppAddress, &resp)
	assert.Equal(t, 200, status)
	assert.True(t, strings.Contains(string(body), `"user_id":"7eP5n"`))
	assert.True(t, strings.Contains(string(body), `"name":"cool app"`))
	assert.Equal(t, "cool app", resp.Data.Name)
	// redirect_uris must be an empty slice (not nil) when no URIs are registered
	assert.Equal(t, []string{}, resp.Data.RedirectUris)
}

func TestGetDeveloperAppUppercaseAddress(t *testing.T) {
	app := testAppWithFixtures(t)

	var resp struct {
		Data dbv1.GetDeveloperAppsRow
	}
	// Uppercase address should still find the app (case-insensitive lookup)
	status, _ := testGet(t, app, "/v1/developer_apps/0x7D7B6B7A97D1DEEFE3A1CCC5A13C48E8F055E0B6", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, "cool app", resp.Data.Name)
}

func TestGetDeveloperAppMixedCaseAddress(t *testing.T) {
	app := testAppWithFixtures(t)

	var resp struct {
		Data dbv1.GetDeveloperAppsRow
	}
	// Mixed-case address should still find the app (case-insensitive lookup)
	status, _ := testGet(t, app, "/v1/developer_apps/0x7d7B6B7A97d1deEFe3A1ccc5A13c48E8F055e0B6", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, "cool app", resp.Data.Name)
}

func TestGetDeveloperAppWithoutHexPrefix(t *testing.T) {
	app := testAppWithFixtures(t)

	var resp struct {
		Data dbv1.GetDeveloperAppsRow
	}
	// Address without 0x prefix should still find the app
	status, _ := testGet(t, app, "/v1/developer_apps/7D7B6B7A97D1DEEFE3A1CCC5A13C48E8F055E0B6", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, "cool app", resp.Data.Name)
}
