package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"testing"

	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1PlaylistTracks(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id": 1,
				"handle":  "user1",
				"name":    "User 1",
			},
		},
		"tracks": []map[string]any{
			{
				"track_id": 1,
				"owner_id": 1,
				"title":    "Track 1",
			},
			{
				"track_id": 2,
				"owner_id": 1,
				"title":    "Track 2",
			},
			{
				"track_id": 3,
				"owner_id": 1,
				"title":    "Track 3",
			},
			{
				"track_id": 4,
				"owner_id": 1,
				"title":    "Track 4",
			},
			{
				"track_id":        5,
				"owner_id":        1,
				"title":           "Track 5",
				"is_stream_gated": true,
			},
		},
		"playlists": []map[string]any{
			{
				"playlist_id":       1,
				"playlist_owner_id": 1,
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
					},
				},
			},
		},
	}
	database.Seed(app.pool.Replicas[0], fixtures)

	// Log playlist_contents for all rows in the playlists table
	rows, err := app.pool.Replicas[0].Query(context.Background(), "SELECT playlist_id, playlist_contents FROM playlists")
	if err != nil {
		t.Fatalf("Failed to query playlists: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var playlistID int
		var playlistContents sql.NullString

		err := rows.Scan(&playlistID, &playlistContents)
		if err != nil {
			t.Fatalf("Failed to scan playlist row: %v", err)
		}

		fmt.Printf("Playlist ID: %d, Contents: %s\n", playlistID, playlistContents.String)

		// Parse and pretty-print the JSON if it's valid
		if playlistContents.Valid && playlistContents.String != "" {
			var contents map[string]interface{}
			if err := json.Unmarshal([]byte(playlistContents.String), &contents); err == nil {
				prettyJSON, _ := json.MarshalIndent(contents, "", "  ")
				fmt.Printf("Pretty-printed contents:\n%s\n", string(prettyJSON))
			}
		}
	}

	if err = rows.Err(); err != nil {
		t.Fatalf("Error iterating playlist rows: %v", err)
	}

	playlistId := trashid.MustEncodeHashID(1)

	{
		status, body := testGet(t, app, "/v1/playlists/"+playlistId+"/tracks", nil)
		assert.Equal(t, 200, status)
		fmt.Println(string(body))
		jsonAssert(t, body, map[string]any{
			"data.#":          4,
			"data.0.track_id": trashid.MustEncodeHashID(1),
			"data.1.track_id": trashid.MustEncodeHashID(2),
			"data.2.track_id": trashid.MustEncodeHashID(3),
			"data.3.track_id": trashid.MustEncodeHashID(4),
		})
	}

	{
		status, body := testGet(t, app, "/v1/playlists/"+playlistId+"/tracks?exclude_gated=false", nil)
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":          1,
			"data.4.track_id": trashid.MustEncodeHashID(5),
		})
	}
}
