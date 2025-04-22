package api

import (
	"strings"
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetDeveloperAppsQueries(t *testing.T) {
	developerApps, err := app.queries.GetDeveloperApps(t.Context(), dbv1.GetDeveloperAppsParams{
		UserID: 1,
	})
	assert.NoError(t, err)
	assert.Len(t, developerApps, 1)
	assert.Equal(t, "0x7d7b6b7a97d1deefe3a1ccc5a13c48e8f055e0b6", developerApps[0].Address)
}

func TestGetDeveloperApp(t *testing.T) {
	var resp struct {
		Data dbv1.GetDeveloperAppsRow
	}
	status, body := testGet(t, "/v1/developer_apps/0x7d7b6b7a97d1deefe3a1ccc5a13c48e8f055e0b6", &resp)
	assert.Equal(t, 200, status)
	assert.True(t, strings.Contains(string(body), `"user_id":"7eP5n"`))
	assert.True(t, strings.Contains(string(body), `"name":"cool app"`))
	assert.Equal(t, "cool app", resp.Data.Name)
}
