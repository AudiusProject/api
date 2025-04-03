package api

import (
	"strings"
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetDeveloperAppsQueries(t *testing.T) {
	userId := int32(1)
	developerApps, err := app.queries.GetDeveloperApps(t.Context(), dbv1.GetDeveloperAppsParams{
		UserID: &userId,
	})
	assert.NoError(t, err)
	assert.Len(t, developerApps, 1)
	assert.Equal(t, "0x7d7b6b7a97d1deefe3a1ccc5a13c48e8f055e0b6", developerApps[0].Address)
}

func TestGetDeveloperApp(t *testing.T) {
	status, body := testGet(t, "/v1/developer_apps/0x7d7b6b7a97d1deefe3a1ccc5a13c48e8f055e0b6")
	assert.Equal(t, 200, status)
	assert.True(t, strings.Contains(string(body), `"user_id":"7eP5n"`))
	assert.True(t, strings.Contains(string(body), `"name": "create-audius-app",`))
}
