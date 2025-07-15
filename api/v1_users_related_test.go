package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersRelated(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := FixtureMap{
		"aggregate_user": []map[string]any{
			{
				"user_id":          1,
				"follower_count":   98,
				"following_count":  25,
				"dominant_genre":   "Electronic",
				"track_save_count": 20,
			},
			{
				"user_id":          2,
				"follower_count":   50,
				"following_count":  15,
				"dominant_genre":   "Electronic",
				"track_save_count": 0,
			},
			{
				"user_id":          3,
				"follower_count":   20,
				"following_count":  10,
				"dominant_genre":   "Electronic",
				"track_save_count": 0,
			},
		},
		"tracks": []map[string]any{
			{
				"track_id":        100,
				"genre":           "Electronic",
				"owner_id":        1,
				"title":           "T1",
				"is_unlisted":     false,
				"is_downloadable": true,
			},
		},
		"users": []map[string]any{
			{
				"user_id":          1,
				"handle":           "rayjacobson",
				"handle_lc":        "rayjacobson",
				"name":             "Ray Jacobson",
				"is_deactivated":   false,
				"wallet":           "0x7d273271690538cf855e5b3002a0dd8c154bb060",
				"playlist_library": []byte(`{"contents":[{"type":"playlist","playlist_id":"123"},{"type":"explore_playlist","playlist_id":"Audio NFTs"},{"type":"folder","id":"bbcae31a-7cd2-4a1a-8b54-fdc979a34435","name":"My Nested Playlists","contents":[{"type":"playlist","playlist_id":"345"}]}]}`),
			},
			{
				"user_id":        2,
				"handle":         "stereosteve",
				"handle_lc":      "stereosteve",
				"is_deactivated": false,
				"wallet":         "0x1234567890abcdef",
			},
			{
				"user_id":        3,
				"handle":         "someseller",
				"handle_lc":      "someseller",
				"is_deactivated": false,
				"wallet":         "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0",
			},
		},
		"follows": []map[string]any{
			{
				"follower_user_id": 1,
				"followee_user_id": 2,
			},
			{
				"follower_user_id": 2,
				"followee_user_id": 1,
			},
			{
				"follower_user_id": 4,
				"followee_user_id": 3,
			},
		},
	}
	createFixtures(app, fixtures)

	var userResponse struct {
		Data []dbv1.FullUser
	}

	{
		status, _ := testGet(t, app, "/v1/users/7eP5n/related", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 2)
		assert.Equal(t, "stereosteve", userResponse.Data[0].Handle.String)
		assert.Equal(t, "someseller", userResponse.Data[1].Handle.String)
	}

	{
		status, _ := testGetWithWallet(
			t, app,
			"/v1/users/7eP5n/related?user_id=7eP5n&filter_followed=true",
			"0x7d273271690538cf855e5b3002a0dd8c154bb060",
			&userResponse,
		)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 1)
		assert.Equal(t, "someseller", userResponse.Data[0].Handle.String)
	}
}
