package api

import (
	"fmt"
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

func (app *ApiServer) v1PlaylistStream(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	playlistId := c.Locals("playlistId").(int)

	playlists, err := app.queries.FullPlaylists(c.Context(), dbv1.FullPlaylistsParams{
		GetPlaylistsParams: dbv1.GetPlaylistsParams{
			MyID: myId,
			Ids:  []int32{int32(playlistId)},
		},
	})
	if err != nil {
		return err
	}

	if len(playlists) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "playlist not found")
	}

	playlist := playlists[0]

	// Build M3U8 playlist content
	var m3u8Builder strings.Builder
	m3u8Builder.WriteString("#EXTM3U\n")
	m3u8Builder.WriteString("#EXT-X-VERSION:3\n")

	// Calculate target duration (max duration of all tracks)
	var maxDuration int32 = 0
	for _, track := range playlist.Tracks {
		if track.Duration.Valid && track.Duration.Int32 > maxDuration {
			maxDuration = track.Duration.Int32
		}
	}
	m3u8Builder.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", maxDuration))

	// Add each track to the playlist
	for _, track := range playlist.Tracks {
		// Check if track has stream access
		if !track.Access.Stream {
			continue
		}

		// Get track duration
		duration := int32(0)
		if track.Duration.Valid {
			duration = track.Duration.Int32
		}

		// Get track title for the EXTINF tag
		title := "Unknown Track"
		if track.Title.Valid {
			title = track.Title.String
		}

		// Generate track stream URL using the track stream endpoint
		trackId := trashid.MustEncodeHashID(int(track.TrackID))
		streamURL := fmt.Sprintf("/v1/tracks/%s/stream", trackId)

		// Write track entry to M3U8
		m3u8Builder.WriteString(fmt.Sprintf("#EXTINF:%d,%s\n", duration, title))
		m3u8Builder.WriteString(streamURL)
		m3u8Builder.WriteString("\n")
	}

	m3u8Builder.WriteString("#EXT-X-ENDLIST\n")

	// Set appropriate content type and return M3U8 content
	c.Set("Content-Type", "application/vnd.apple.mpegurl")
	return c.SendString(m3u8Builder.String())
}
