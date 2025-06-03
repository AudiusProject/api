package dbv1

import (
	"context"
	"fmt"

	"bridgerton.audius.co/trashid"
)

type FullAccountPlaylistOwner struct {
	ID            string `json:"id"`
	Handle        string `json:"handle"`
	IsDeactivated bool   `json:"is_deactivated"`
}

type FullAccountPlaylist struct {
	ID        string `json:"id"`
	IsAlbum   bool   `json:"is_album"`
	Name      string `json:"name"`
	Permalink string `json:"permalink"`

	User FullAccountPlaylistOwner `json:"user"`
}

func (q *Queries) FullAccountPlaylists(ctx context.Context, userID int32) ([]FullAccountPlaylist, error) {
	playlists, err := q.GetAccountPlaylists(ctx, userID)
	if err != nil {
		return nil, err
	}

	fullPlaylists := make([]FullAccountPlaylist, len(playlists))

	for idx, p := range playlists {
		playlistID, err := trashid.EncodeHashId(int(p.PlaylistID))
		if err != nil {
			return nil, err
		}
		userID, err := trashid.EncodeHashId(int(p.UserID))
		if err != nil {
			return nil, err
		}
		var playlistType string
		if p.IsAlbum {
			playlistType = "album"
		} else {
			playlistType = "playlist"
		}
		fullPlaylists[idx] = FullAccountPlaylist{
			ID:        playlistID,
			IsAlbum:   p.IsAlbum,
			Name:      p.PlaylistName.String,
			Permalink: fmt.Sprintf("/%s/%s/%s", p.Handle.String, playlistType, p.Slug),
			User: FullAccountPlaylistOwner{
				ID:            userID,
				Handle:        p.Handle.String,
				IsDeactivated: p.IsDeactivated,
			},
		}
	}

	return fullPlaylists, nil
}
