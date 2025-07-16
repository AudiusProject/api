package api

import (
	"testing"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetUserPlaylists(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": {{"user_id": 1, "handle_lc": "one"}},
		"playlists": {
			{
				"playlist_id":       1,
				"playlist_owner_id": 1,
				"is_album":          false,
				"created_at":        time.Now().AddDate(0, 0, -3),
				"playlist_name":     "one",
			},
			{
				"playlist_id":       2,
				"playlist_owner_id": 1,
				"is_album":          false,
				"created_at":        time.Now().AddDate(0, 0, -2),
				"playlist_name":     "two",
			},
			{
				"playlist_id":       3,
				"playlist_owner_id": 1,
				"is_album":          true,
				"created_at":        time.Now().AddDate(0, 0, -1),
				"playlist_name":     "three",
			},
		},
		"aggregate_playlist": {
			{"playlist_id": 1, "repost_count": 10, "save_count": 5},
			{"playlist_id": 2, "repost_count": 20, "save_count": 10},
		},
	}
	database.Seed(app.pool, fixtures)

	var userPlaylistsResponse struct {
		Data []dbv1.FullPlaylist
	}

	{
		status, body := testGet(t, app, "/v1/full/users/handle/one/playlists", &userPlaylistsResponse)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.0.id": trashid.MustEncodeHashID(2),
			"data.1.id": trashid.MustEncodeHashID(1),
		})
	}
	{
		status, body := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(1)+"/playlists", &userPlaylistsResponse)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.0.id": trashid.MustEncodeHashID(2),
			"data.1.id": trashid.MustEncodeHashID(1),
		})
	}
}

func TestGetUserPlaylists_SortRecentDesc(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": {{"user_id": 1, "handle_lc": "one"}},
		"playlists": {
			{
				"playlist_id":       1,
				"playlist_owner_id": 1,
				"is_album":          false,
				"created_at":        time.Now().AddDate(0, 0, -3),
				"playlist_name":     "one",
			},
			{
				"playlist_id":       2,
				"playlist_owner_id": 1,
				"is_album":          false,
				"created_at":        time.Now().AddDate(0, 0, -2),
				"playlist_name":     "two",
			},
			{
				"playlist_id":       3,
				"playlist_owner_id": 1,
				"is_album":          true,
				"created_at":        time.Now().AddDate(0, 0, -1),
				"playlist_name":     "three",
			},
		},
		"aggregate_playlist": {
			{"playlist_id": 1, "repost_count": 10, "save_count": 5},
			{"playlist_id": 2, "repost_count": 20, "save_count": 10},
		},
	}
	database.Seed(app.pool, fixtures)

	var userPlaylistsResponse struct {
		Data []dbv1.FullPlaylist
	}

	status, body := testGet(t, app, "/v1/full/users/handle/one/playlists?sort_method=recent&sort_direction=desc", &userPlaylistsResponse)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(2),
		"data.1.id": trashid.MustEncodeHashID(1),
	})
}

func TestGetUserPlaylists_SortPopularAsc(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": {{"user_id": 1, "handle_lc": "one"}},
		"playlists": {
			{
				"playlist_id":       1,
				"playlist_owner_id": 1,
				"is_album":          false,
				"created_at":        time.Now().AddDate(0, 0, -3),
				"playlist_name":     "one",
			},
			{
				"playlist_id":       2,
				"playlist_owner_id": 1,
				"is_album":          false,
				"created_at":        time.Now().AddDate(0, 0, -2),
				"playlist_name":     "two",
			},
			{
				"playlist_id":       3,
				"playlist_owner_id": 1,
				"is_album":          true,
				"created_at":        time.Now().AddDate(0, 0, -1),
				"playlist_name":     "three",
			},
		},
		"aggregate_playlist": {
			{"playlist_id": 1, "repost_count": 10, "save_count": 5},
			{"playlist_id": 2, "repost_count": 20, "save_count": 10},
		},
	}
	database.Seed(app.pool, fixtures)

	var userPlaylistsResponse struct {
		Data []dbv1.FullPlaylist
	}

	status, body := testGet(t, app, "/v1/full/users/handle/one/playlists?sort_method=popular&sort_direction=asc", &userPlaylistsResponse)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(1),
		"data.1.id": trashid.MustEncodeHashID(2),
	})
}

func TestGetUserPlaylists_FilterPlaylistsPublic(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": {{"user_id": 1, "handle_lc": "one"}},
		"playlists": {
			{
				"playlist_id":       1,
				"playlist_owner_id": 1,
				"is_album":          false,
				"is_private":        true,
				"created_at":        time.Now().AddDate(0, 0, -3),
				"playlist_name":     "one",
			},
			{
				"playlist_id":       2,
				"playlist_owner_id": 1,
				"is_album":          false,
				"is_private":        false,
				"created_at":        time.Now().AddDate(0, 0, -2),
				"playlist_name":     "two",
			},
			{
				"playlist_id":       3,
				"playlist_owner_id": 1,
				"is_album":          false,
				"is_private":        true,
				"created_at":        time.Now().AddDate(0, 0, -1),
				"playlist_name":     "three",
			},
		},
		"aggregate_playlist": {
			{"playlist_id": 1, "repost_count": 10, "save_count": 5},
			{"playlist_id": 2, "repost_count": 20, "save_count": 10},
		},
	}
	database.Seed(app.pool, fixtures)

	var userPlaylistsResponse struct {
		Data []dbv1.FullPlaylist
	}

	status, body := testGet(t, app, "/v1/full/users/handle/one/playlists?filter_playlists=public", &userPlaylistsResponse)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#":    1,
		"data.0.id": trashid.MustEncodeHashID(2),
	})
}

func TestGetUserPlaylists_FilterPlaylistsPrivate(t *testing.T) {
	app := emptyTestApp(t)
	fixtures := database.FixtureMap{
		"users": {{"user_id": 1, "handle_lc": "one"}},
		"playlists": {
			{
				"playlist_id":       1,
				"playlist_owner_id": 1,
				"is_album":          false,
				"is_private":        true,
				"created_at":        time.Now().AddDate(0, 0, -3),
				"playlist_name":     "one",
			},
			{
				"playlist_id":       2,
				"playlist_owner_id": 1,
				"is_album":          false,
				"is_private":        false,
				"created_at":        time.Now().AddDate(0, 0, -2),
				"playlist_name":     "two",
			},
			{
				"playlist_id":       3,
				"playlist_owner_id": 1,
				"is_album":          false,
				"is_private":        true,
				"created_at":        time.Now().AddDate(0, 0, -1),
				"playlist_name":     "three",
			},
		},
		"aggregate_playlist": {
			{"playlist_id": 1, "repost_count": 10, "save_count": 5},
			{"playlist_id": 2, "repost_count": 20, "save_count": 10},
		},
	}
	database.Seed(app.pool, fixtures)

	var userPlaylistsResponse struct {
		Data []dbv1.FullPlaylist
	}

	status, body := testGet(t, app, "/v1/full/users/handle/one/playlists?filter_playlists=private", &userPlaylistsResponse)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#":    2,
		"data.0.id": trashid.MustEncodeHashID(3),
		"data.1.id": trashid.MustEncodeHashID(1),
	})
}
