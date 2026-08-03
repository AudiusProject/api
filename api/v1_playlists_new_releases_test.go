package api

import (
	"testing"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetPlaylistsNewReleases_Albums(t *testing.T) {
	app := emptyTestApp(t)
	now := time.Now()
	fixtures := database.FixtureMap{
		"users": {{"user_id": 1, "handle_lc": "one"}},
		"playlists": {
			// newest album
			{
				"playlist_id":       1,
				"playlist_owner_id": 1,
				"is_album":          true,
				"is_private":        false,
				"release_date":      now.AddDate(0, 0, -1),
				"created_at":        now.AddDate(0, 0, -1),
				"playlist_name":     "one",
			},
			// older album with stronger engagement
			{
				"playlist_id":       2,
				"playlist_owner_id": 1,
				"is_album":          true,
				"is_private":        false,
				"release_date":      now.AddDate(0, 0, -10),
				"created_at":        now.AddDate(0, 0, -10),
				"playlist_name":     "two",
			},
			// recent album without enough social proof (excluded)
			{
				"playlist_id":       7,
				"playlist_owner_id": 1,
				"is_album":          true,
				"is_private":        false,
				"release_date":      now.AddDate(0, 0, -2),
				"created_at":        now.AddDate(0, 0, -2),
				"playlist_name":     "seven",
			},
			// non-album playlist (excluded when type=album)
			{
				"playlist_id":       3,
				"playlist_owner_id": 1,
				"is_album":          false,
				"is_private":        false,
				"release_date":      now,
				"created_at":        now,
				"playlist_name":     "three",
			},
			// private album (excluded)
			{
				"playlist_id":       4,
				"playlist_owner_id": 1,
				"is_album":          true,
				"is_private":        true,
				"release_date":      now,
				"created_at":        now,
				"playlist_name":     "four",
			},
			// outside of 90-day window (excluded)
			{
				"playlist_id":       5,
				"playlist_owner_id": 1,
				"is_album":          true,
				"is_private":        false,
				"release_date":      now.AddDate(0, 0, -120),
				"created_at":        now.AddDate(0, 0, -120),
				"playlist_name":     "five",
			},
			// future release (excluded until release_date has passed)
			{
				"playlist_id":       6,
				"playlist_owner_id": 1,
				"is_album":          true,
				"is_private":        false,
				"release_date":      now.AddDate(0, 0, 10),
				"created_at":        now,
				"playlist_name":     "six",
			},
		},
		"aggregate_playlist": {
			{"playlist_id": 1, "is_album": true, "save_count": 3, "repost_count": 2},
			{"playlist_id": 2, "is_album": true, "save_count": 6, "repost_count": 2},
			{"playlist_id": 7, "is_album": true, "save_count": 2, "repost_count": 2},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	var resp struct {
		Data []dbv1.Playlist
	}

	status, body := testGet(t, app, "/v1/playlists/new-releases?type=album", &resp)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(2),
		"data.1.id": trashid.MustEncodeHashID(1),
	})
	assert.Len(t, resp.Data, 2)
}

func TestGetPlaylistsNewReleases_PlaylistsRemainChronological(t *testing.T) {
	app := emptyTestApp(t)
	now := time.Now()
	fixtures := database.FixtureMap{
		"users": {{"user_id": 1, "handle_lc": "one"}},
		"playlists": {
			{
				"playlist_id":       1,
				"playlist_owner_id": 1,
				"is_album":          false,
				"is_private":        false,
				"release_date":      now.AddDate(0, 0, -1),
				"created_at":        now.AddDate(0, 0, -1),
				"playlist_name":     "newer playlist",
			},
			{
				"playlist_id":       2,
				"playlist_owner_id": 1,
				"is_album":          false,
				"is_private":        false,
				"release_date":      now.AddDate(0, 0, -10),
				"created_at":        now.AddDate(0, 0, -10),
				"playlist_name":     "older playlist",
			},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	var resp struct {
		Data []dbv1.Playlist
	}

	status, body := testGet(t, app, "/v1/playlists/new-releases?type=playlist", &resp)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(1),
		"data.1.id": trashid.MustEncodeHashID(2),
	})
	assert.Len(t, resp.Data, 2)
}
