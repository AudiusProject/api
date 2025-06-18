package api

import (
	"testing"

	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrackCommentNotificationSetting(t *testing.T) {
	app := testAppWithFixtures(t)
	fixtures := FixtureMap{
		"comment_notification_settings": []map[string]any{
			{
				"user_id":   1,
				"entity_id": 1,
				"is_muted":  false,
			},
		},
	}
	createFixtures(app, fixtures)
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
	app := testAppWithFixtures(t)
	fixtures := FixtureMap{
		"comment_notification_settings": []map[string]any{
			{
				"user_id":   1,
				"entity_id": 2,
				"is_muted":  true,
			},
		},
	}
	createFixtures(app, fixtures)
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
	app := testAppWithFixtures(t)
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
