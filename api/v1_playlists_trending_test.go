package api

import (
	"fmt"
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrendingPlaylists(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"playlists":                []map[string]any{},
		"tracks":                   []map[string]any{},
		"playlist_tracks":          []map[string]any{},
		"playlist_trending_scores": []map[string]any{},
		"users":                    []map[string]any{},
	}
	// Make sure we have enough unique owners
	for _, userID := range []int{1, 2, 3, 4, 5, 7, 8} {
		fixtures["users"] = append(fixtures["users"], map[string]any{
			"user_id":   userID,
			"handle":    fmt.Sprintf("testuser%d", userID),
			"handle_lc": fmt.Sprintf("testuser%d", userID),
			"wallet":    fmt.Sprintf("0x%064x", userID),
		})
	}
	// 5 tracks with unique owners
	for _, trackID := range []int{1, 2, 3, 4, 5} {
		fixtures["tracks"] = append(fixtures["tracks"], map[string]any{
			"track_id": trackID,
			"owner_id": trackID,
		})
	}
	// Track with same owner as another track
	fixtures["tracks"] = append(fixtures["tracks"], map[string]any{
		"track_id": 6,
		"owner_id": 1,
	})
	// 8 playlists owned by different users
	for _, playlistID := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		fixtures["playlists"] = append(fixtures["playlists"], map[string]any{
			"playlist_id":       playlistID,
			"playlist_owner_id": playlistID, // Just mapping to same user id
			"is_current":        true,
			"is_delete":         false,
			"is_private":        false,
			"is_album":          false,
		})
		fixtures["playlist_trending_scores"] = append(fixtures["playlist_trending_scores"], map[string]any{
			"playlist_id": playlistID,
			"type":        "PLAYLISTS",
			"version":     "pnagD",
			"time_range":  "week",
			"score":       100 - playlistID,
		})
	}
	// Playlist 1 (id 2) is an album, so ineligible for trending
	fixtures["playlists"][1]["is_album"] = true
	// These are all elgibile based on uniqueness criteria
	for _, playlistID := range []int{1, 2, 3, 4, 7, 8} {
		for _, trackID := range []int{1, 2, 3, 4, 5} {
			fixtures["playlist_tracks"] = append(fixtures["playlist_tracks"], map[string]any{
				"playlist_id": playlistID,
				"track_id":    trackID,
			})
		}
	}
	// These are ineligible on uniqueness criteria
	for _, trackID := range []int{1, 2, 3, 4} {
		fixtures["playlist_tracks"] = append(fixtures["playlist_tracks"], map[string]any{
			"playlist_id": 5,
			"track_id":    trackID,
		})
	}
	for _, trackID := range []int{1, 2, 3, 4, 6} {
		fixtures["playlist_tracks"] = append(fixtures["playlist_tracks"], map[string]any{
			"playlist_id": 6,
			"track_id":    trackID,
		})
	}

	database.Seed(app.pool.Replicas[0], fixtures)
	status, body := testGet(t, app, "/v1/playlists/trending?limit=5", nil)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#": 5,
	})
	jsonAssert(t, body, map[string]any{
		"data.0.id": trashid.MustEncodeHashID(1),
	})
	jsonAssert(t, body, map[string]any{
		"data.1.id": trashid.MustEncodeHashID(3),
	})
	jsonAssert(t, body, map[string]any{
		"data.2.id": trashid.MustEncodeHashID(4),
	})
	jsonAssert(t, body, map[string]any{
		"data.3.id": trashid.MustEncodeHashID(7),
	})
	jsonAssert(t, body, map[string]any{
		"data.4.id": trashid.MustEncodeHashID(8),
	})
}

func TestGetTrendingPlaylistsTrackLimit(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"playlists":                []map[string]any{},
		"tracks":                   []map[string]any{},
		"playlist_tracks":          []map[string]any{},
		"playlist_trending_scores": []map[string]any{},
		"users":                    []map[string]any{},
	}
	// Make sure we have enough unique owners
	for _, userID := range []int{1, 2, 3, 4, 5, 6} {
		fixtures["users"] = append(fixtures["users"], map[string]any{
			"user_id":   userID,
			"handle":    fmt.Sprintf("testuser%d", userID),
			"handle_lc": fmt.Sprintf("testuser%d", userID),
			"wallet":    fmt.Sprintf("0x%064x", userID),
		})
	}
	// 5 tracks with unique owners
	for _, trackID := range []int{1, 2, 3, 4, 5, 6} {
		fixtures["tracks"] = append(fixtures["tracks"], map[string]any{
			"track_id": trackID,
			"owner_id": trackID,
		})
	}
	fixtures["playlists"] = append(fixtures["playlists"], map[string]any{
		"playlist_id":       1,
		"playlist_owner_id": 1,
		"is_current":        true,
		"is_delete":         false,
		"is_private":        false,
		"playlist_contents": map[string]any{
			"track_ids": []map[string]any{
				{
					"track":         1,
					"time":          1,
					"metadata_time": 1,
				},
				{
					"track":         2,
					"time":          2,
					"metadata_time": 2,
				},
				{
					"track":         3,
					"time":          3,
					"metadata_time": 3,
				},
				{
					"track":         4,
					"time":          4,
					"metadata_time": 4,
				},
				{
					"track":         5,
					"time":          5,
					"metadata_time": 5,
				},
				{
					"track":         6,
					"time":          6,
					"metadata_time": 6,
				},
			},
		},
		"is_album": false,
	})
	fixtures["playlist_trending_scores"] = append(fixtures["playlist_trending_scores"], map[string]any{
		"playlist_id": 1,
		"type":        "PLAYLISTS",
		"version":     "pnagD",
		"time_range":  "week",
		"score":       100,
	})
	for _, trackID := range []int{1, 2, 3, 4, 5, 6} {
		fixtures["playlist_tracks"] = append(fixtures["playlist_tracks"], map[string]any{
			"playlist_id": 1,
			"track_id":    trackID,
		})
	}

	database.Seed(app.pool.Replicas[0], fixtures)
	status, body := testGet(t, app, "/v1/playlists/trending?limit=1", nil)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#": 1,
	})
	jsonAssert(t, body, map[string]any{
		"data.0.tracks.#":    5,
		"data.0.track_count": 6,
	})
}

func TestGetTrendingPlaylists_Albums(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"playlists":                []map[string]any{},
		"tracks":                   []map[string]any{},
		"playlist_tracks":          []map[string]any{},
		"playlist_trending_scores": []map[string]any{},
		"users":                    []map[string]any{},
	}
	// Make sure we have enough unique owners
	for _, userID := range []int{1, 2, 3, 4, 5, 7, 8} {
		fixtures["users"] = append(fixtures["users"], map[string]any{
			"user_id":   userID,
			"handle":    fmt.Sprintf("testuser%d", userID),
			"handle_lc": fmt.Sprintf("testuser%d", userID),
			"wallet":    fmt.Sprintf("0x%064x", userID),
		})
	}
	// 5 tracks with unique owners
	for _, trackID := range []int{1, 2, 3, 4, 5} {
		fixtures["tracks"] = append(fixtures["tracks"], map[string]any{
			"track_id": trackID,
			"owner_id": trackID,
		})
	}
	// Track with same owner as another track
	fixtures["tracks"] = append(fixtures["tracks"], map[string]any{
		"track_id": 6,
		"owner_id": 1,
	})
	// 8 playlists owned by different users
	for _, playlistID := range []int{1, 2, 3, 4, 5, 6, 7, 8} {
		fixtures["playlists"] = append(fixtures["playlists"], map[string]any{
			"playlist_id":       playlistID,
			"playlist_owner_id": playlistID, // Just mapping to same user id
			"is_current":        true,
			"is_delete":         false,
			"is_private":        false,
			"is_album":          true,
		})
		fixtures["playlist_trending_scores"] = append(fixtures["playlist_trending_scores"], map[string]any{
			"playlist_id": playlistID,
			"type":        "PLAYLISTS",
			"version":     "pnagD",
			"time_range":  "week",
			"score":       100 - playlistID,
		})
	}
	// Playlist 1 (id 2) is a playlist, so ineligible for trending album
	fixtures["playlists"][1]["is_album"] = false
	// These are all elgibile based on uniqueness criteria
	for _, playlistID := range []int{1, 2, 3, 4, 7, 8} {
		for _, trackID := range []int{1, 2, 3, 4, 5, 6} {
			fixtures["playlist_tracks"] = append(fixtures["playlist_tracks"], map[string]any{
				"playlist_id": playlistID,
				"track_id":    trackID,
			})
		}
	}

	database.Seed(app.pool.Replicas[0], fixtures)
	{
		status, body := testGet(t, app, "/v1/playlists/trending?limit=5&type=album", nil)
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#": 5,
		})
		jsonAssert(t, body, map[string]any{
			"data.0.id": trashid.MustEncodeHashID(1),
		})
		jsonAssert(t, body, map[string]any{
			"data.1.id": trashid.MustEncodeHashID(3),
		})
		jsonAssert(t, body, map[string]any{
			"data.2.id": trashid.MustEncodeHashID(4),
		})
		jsonAssert(t, body, map[string]any{
			"data.3.id": trashid.MustEncodeHashID(7),
		})
		jsonAssert(t, body, map[string]any{
			"data.4.id": trashid.MustEncodeHashID(8),
		})
	}
	{
		status, body := testGet(t, app, "/v1/playlists/trending?limit=5&type=playlist", nil)
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			// Get back the one playlist that is a playlist
			"data.#": 1,
		})
	}
}
