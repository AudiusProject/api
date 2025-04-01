package queries

import (
	"context"

	"bridgerton.audius.co/trashid"
)

type FullPlaylist struct {
	GetPlaylistsRow

	ID      string      `json:"id"`
	Artwork SquareImage `json:"artwork"`
	UserID  string      `json:"user_id"`
	User    FullUser    `json:"user"`
}

func (q *Queries) FullPlaylists(ctx context.Context, arg GetPlaylistsParams) ([]FullPlaylist, error) {
	rawPlaylists, err := q.GetPlaylists(ctx, arg)
	if err != nil {
		return nil, err
	}

	// todo get track ids from playlist_contents
	// fetch full tracks

	userIds := make([]int32, len(rawPlaylists))
	for idx, p := range rawPlaylists {
		userIds[idx] = p.PlaylistOwnerID
	}

	users, err := q.FullUsers(ctx, GetUsersParams{
		MyID: arg.MyID,
		Ids:  userIds,
	})
	if err != nil {
		return nil, err
	}

	userMap := map[int32]FullUser{}
	for _, user := range users {
		userMap[user.UserID] = user
	}

	fullPlaylists := make([]FullPlaylist, 0, len(rawPlaylists))
	for _, playlist := range rawPlaylists {
		id, _ := trashid.EncodeHashId(int(playlist.PlaylistID))
		user, ok := userMap[playlist.PlaylistOwnerID]

		// GetUser will omit deactivated users
		// so skip tracks if user doesn't come back.
		// .. todo: in get_tracks query we should join users and filter out tracks if user is deactivated at query time.
		if !ok {
			continue
		}

		fullPlaylists = append(fullPlaylists, FullPlaylist{
			GetPlaylistsRow: playlist,
			ID:              id,
			// Artwork:         squareImageStruct(track.CoverArtSizes, track.CoverArt),
			User:   user,
			UserID: user.ID,
		})
	}

	return fullPlaylists, nil
}
