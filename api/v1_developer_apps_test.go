package api

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetDeveloperAppsQueries(t *testing.T) {
	userId := int32(1)
	developerApps, err := app.queries.GetDeveloperAppsByUser(t.Context(), &userId)
	assert.NoError(t, err)
	assert.Len(t, developerApps, 1)
	assert.Equal(t, "0x7d7b6b7a97d1deefe3a1ccc5a13c48e8f055e0b6", developerApps[0].Address)
}

func TestGetDeveloperApp(t *testing.T) {
	status, body := testGet(t, "/v1/developer_apps/0x7d7b6b7a97d1deefe3a1ccc5a13c48e8f055e0b6")
	assert.Equal(t, 200, status)
	assert.True(t, strings.Contains(string(body), `"user_id":"7eP5n"`))
}
