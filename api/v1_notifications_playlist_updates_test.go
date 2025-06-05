package api

import (
	"fmt"
	"testing"
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1NotificationsPlaylistUpdates(t *testing.T) {
	app := emptyTestApp(t)
	userID := int32(4001)
	playlistID := int32(4001)
	playlist2ID := int32(4002)
	now := time.Now().UTC()

	fixtures := FixtureSet{
		blocks: []map[string]any{
			{}, // default block
		},
		users: []map[string]any{
			{
				"user_id": userID,
				"handle":  "playlistuser",
			},
		},
		playlists: []map[string]any{
			{
				"playlist_id":       playlistID,
				"playlist_owner_id": userID,
				"playlist_name":     "My Playlist",
				"is_current":        true,
				"is_delete":         false,
				"updated_at":        now,
			},
			{
				"playlist_id":       playlist2ID,
				"playlist_owner_id": userID,
				"playlist_name":     "My Playlist2",
				"is_current":        true,
				"is_delete":         false,
				"updated_at":        now,
			},
		},
		saves: []map[string]any{
			{
				"user_id":      userID,
				"save_item_id": playlistID,
				"save_type":    "playlist",
				"created_at":   now.Add(-time.Hour),
			},
		},
		playlist_seen: []map[string]any{
			{
				"user_id":     userID,
				"playlist_id": playlist2ID,
				"seen_at":     now,
			},
		},
	}

	createFixtures(app, fixtures)

	status, body := testGet(t, app, fmt.Sprintf("/v1/notifications/%s/playlist_updates", trashid.MustEncodeHashID(int(userID))))
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.playlist_updates.0.playlist_id":  trashid.MustEncodeHashID(int(playlistID)),
		"data.playlist_updates.0.updated_at":   float64(now.Unix()),
		"data.playlist_updates.0.last_seen_at": nil,
		"data.playlist_updates.1.playlist_id":  nil, // playlist2ID is not included because it was seen
	})
}
