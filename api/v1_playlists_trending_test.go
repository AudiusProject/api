package api

import (
	"fmt"
	"testing"

	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrendingPlaylists(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := FixtureMap{
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

	createFixtures(app, fixtures)
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
