package api

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"net/http/httptest"
	"testing"

	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func requestWithBasicAuth(t *testing.T, app *ApiServer, method string, path string, privateKey string, body []byte) (int, []byte) {
	t.Helper()

	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("user:%s", privateKey)))
	req.Header.Set("Authorization", "Basic "+auth)

	res, err := app.Test(req, -1)
	assert.NoError(t, err)

	resBody, _ := io.ReadAll(res.Body)
	return res.StatusCode, resBody
}

func TestEngagementMutationRoutes(t *testing.T) {
	app := emptyTestApp(t)

	// Shared dev key used in other mutator tests.
	privateKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"

	userID := trashid.MustEncodeHashID(101)
	trackID := trashid.MustEncodeHashID(102)
	playlistID := trashid.MustEncodeHashID(103)

	cases := []struct {
		name        string
		method      string
		path        string
		body        []byte
		expectedErr string
	}{
		{
			name:        "post user follow",
			method:      "POST",
			path:        "/v1/full/users/" + userID + "/follow",
			expectedErr: "Failed to follow user",
		},
		{
			name:        "delete user follow",
			method:      "DELETE",
			path:        "/v1/full/users/" + userID + "/follow",
			expectedErr: "Failed to unfollow user",
		},
		{
			name:        "post user mute",
			method:      "POST",
			path:        "/v1/full/users/" + userID + "/mute",
			expectedErr: "Failed to mute user",
		},
		{
			name:        "delete user mute",
			method:      "DELETE",
			path:        "/v1/full/users/" + userID + "/mute",
			expectedErr: "Failed to unmute user",
		},
		{
			name:        "post user subscribe",
			method:      "POST",
			path:        "/v1/full/users/" + userID + "/subscribe",
			expectedErr: "Failed to subscribe to user",
		},
		{
			name:        "delete user subscribe",
			method:      "DELETE",
			path:        "/v1/full/users/" + userID + "/subscribe",
			expectedErr: "Failed to unsubscribe from user",
		},
		{
			name:        "post track favorite",
			method:      "POST",
			path:        "/v1/full/tracks/" + trackID + "/favorites",
			body:        []byte(`{"is_save_of_repost":true}`),
			expectedErr: "Failed to favorite track",
		},
		{
			name:        "delete track favorite",
			method:      "DELETE",
			path:        "/v1/full/tracks/" + trackID + "/favorites",
			expectedErr: "Failed to unfavorite track",
		},
		{
			name:        "post track share",
			method:      "POST",
			path:        "/v1/full/tracks/" + trackID + "/shares",
			expectedErr: "Failed to share track",
		},
		{
			name:        "post track download",
			method:      "POST",
			path:        "/v1/full/tracks/" + trackID + "/downloads",
			body:        []byte(`{"city":"Los Angeles","region":"CA","country":"US"}`),
			expectedErr: "Failed to record track download",
		},
		{
			name:        "delete track repost",
			method:      "DELETE",
			path:        "/v1/full/tracks/" + trackID + "/reposts",
			expectedErr: "Failed to unrepost track",
		},
		{
			name:        "post playlist favorite",
			method:      "POST",
			path:        "/v1/full/playlists/" + playlistID + "/favorites",
			body:        []byte(`{"is_save_of_repost":true}`),
			expectedErr: "Failed to favorite playlist",
		},
		{
			name:        "delete playlist favorite",
			method:      "DELETE",
			path:        "/v1/full/playlists/" + playlistID + "/favorites",
			expectedErr: "Failed to unfavorite playlist",
		},
		{
			name:        "post playlist repost",
			method:      "POST",
			path:        "/v1/full/playlists/" + playlistID + "/reposts",
			body:        []byte(`{"is_repost_of_repost":true}`),
			expectedErr: "Failed to repost playlist",
		},
		{
			name:        "delete playlist repost",
			method:      "DELETE",
			path:        "/v1/full/playlists/" + playlistID + "/reposts",
			expectedErr: "Failed to unrepost playlist",
		},
		{
			name:        "post playlist share",
			method:      "POST",
			path:        "/v1/full/playlists/" + playlistID + "/shares",
			expectedErr: "Failed to share playlist",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := requestWithBasicAuth(t, app, tc.method, tc.path, privateKey, tc.body)
			assert.Equal(t, 500, status, "response body: %s", string(body))
			jsonAssert(t, body, map[string]any{
				"error": tc.expectedErr,
			})
		})
	}
}

func TestEntityMutationRoutes(t *testing.T) {
	app := emptyTestApp(t)

	privateKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	userID := trashid.MustEncodeHashID(201)
	trackID := trashid.MustEncodeHashID(202)
	playlistID := trashid.MustEncodeHashID(203)

	cases := []struct {
		name         string
		method       string
		path         string
		body         []byte
		expectStatus int
		expectedErr  string
	}{
		{
			name:         "post track",
			method:       "POST",
			path:         "/v1/full/tracks",
			body:         []byte(`{"title":"Track","genre":"Electronic","track_cid":"track-cid"}`),
			expectStatus: 500,
			expectedErr:  "Failed to create track",
		},
		{
			name:         "put track with empty update",
			method:       "PUT",
			path:         "/v1/full/tracks/" + trackID,
			body:         []byte(`{}`),
			expectStatus: 400,
			expectedErr:  "At least one field must be provided for update",
		},
		{
			name:         "delete track",
			method:       "DELETE",
			path:         "/v1/full/tracks/" + trackID,
			expectStatus: 500,
			expectedErr:  "Failed to delete track",
		},
		{
			name:         "post playlist",
			method:       "POST",
			path:         "/v1/full/playlists",
			body:         []byte(`{"playlist_name":"Playlist"}`),
			expectStatus: 500,
			expectedErr:  "Failed to create playlist",
		},
		{
			name:         "put playlist with empty update",
			method:       "PUT",
			path:         "/v1/full/playlists/" + playlistID,
			body:         []byte(`{}`),
			expectStatus: 400,
			expectedErr:  "At least one field must be provided for update",
		},
		{
			name:         "delete playlist",
			method:       "DELETE",
			path:         "/v1/full/playlists/" + playlistID,
			expectStatus: 500,
			expectedErr:  "Failed to delete playlist",
		},
		{
			name:         "post user",
			method:       "POST",
			path:         "/v1/full/users",
			body:         []byte(`{"handle":"newuser","wallet":"0x1111111111111111111111111111111111111111"}`),
			expectStatus: 500,
			expectedErr:  "Failed to create user",
		},
		{
			name:         "put user with empty update",
			method:       "PUT",
			path:         "/v1/full/users/" + userID,
			body:         []byte(`{}`),
			expectStatus: 400,
			expectedErr:  "At least one field must be provided for update",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := requestWithBasicAuth(t, app, tc.method, tc.path, privateKey, tc.body)
			assert.Equal(t, tc.expectStatus, status, "response body: %s", string(body))
			jsonAssert(t, body, map[string]any{
				"error": tc.expectedErr,
			})
		})
	}
}
