package api

import (
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrackCommentNotificationSetting(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "wallet": "0x7d273271690538cf855e5b3002a0dd8c154bb060"},
		},
		"comment_notification_settings": []map[string]any{
			{
				"user_id":   1,
				"entity_id": 1,
				"is_muted":  false,
			},
		},
	}
	database.Seed(app.pool, fixtures)
	status, body := testGetWithWallet(
		t, app,
		"/v1/tracks/"+trashid.MustEncodeHashID(1)+"/comment_notification_setting?user_id="+trashid.MustEncodeHashID(1),
		"0x7d273271690538cf855e5b3002a0dd8c154bb060",
	)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.is_muted": false,
	})
}

func TestGetTrackCommentNotificationSettingMuted(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "wallet": "0x7d273271690538cf855e5b3002a0dd8c154bb060"},
		},
		"comment_notification_settings": []map[string]any{
			{
				"user_id":   1,
				"entity_id": 2,
				"is_muted":  true,
			},
		},
	}
	database.Seed(app.pool, fixtures)
	status, body := testGetWithWallet(
		t, app,
		"/v1/tracks/"+trashid.MustEncodeHashID(2)+"/comment_notification_setting?user_id="+trashid.MustEncodeHashID(1),
		"0x7d273271690538cf855e5b3002a0dd8c154bb060",
	)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.is_muted": true,
	})
}

func TestGetTrackCommentNotificationSettingNotFound(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{"user_id": 1, "wallet": "0x7d273271690538cf855e5b3002a0dd8c154bb060"},
		},
	}
	database.Seed(app.pool, fixtures)
	status, body := testGetWithWallet(
		t, app,
		"/v1/tracks/"+trashid.MustEncodeHashID(999)+"/comment_notification_setting?user_id="+trashid.MustEncodeHashID(1),
		"0x7d273271690538cf855e5b3002a0dd8c154bb060",
	)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.is_muted": false,
	})
}
