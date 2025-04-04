package dbv1

import (
	"context"

	"bridgerton.audius.co/trashid"
	"golang.org/x/sync/errgroup"
)

type FullPlaylist struct {
	GetPlaylistsRow

	ID      string      `json:"id"`
	Artwork SquareImage `json:"artwork"`
	UserID  string      `json:"user_id"`
	User    FullUser    `json:"user"`
	Tracks  []FullTrack `json:"tracks"`
}

type MinPlaylist struct {
	ID               string      `json:"id"`
	PlaylistName     *string     `json:"playlist_name"`
	PlaylistOwnerID  int32       `json:"playlist_owner_id"`
	PlaylistID       int32       `json:"playlist_id"`
	Artwork          SquareImage `json:"artwork"`
	Description      *string     `json:"description"`
	PlaylistContents interface{} `json:"playlist_contents"`
	IsAlbum          bool        `json:"is_album"`
	IsPrivate        bool        `json:"is_private"`
	FavoriteCount    int32       `json:"favorite_count"`
	RepostCount      int32       `json:"repost_count"`
	UserID           string      `json:"user_id"`
	User             MinUser     `json:"user"`
	Tracks           []MinTrack  `json:"tracks"`
}

func (q *Queries) FullPlaylists(ctx context.Context, arg GetPlaylistsParams) ([]FullPlaylist, error) {
	rawPlaylists, err := q.GetPlaylists(ctx, arg)
	if err != nil {
		return nil, err
	}

	// pluck user + track IDs
	trackIds := []int32{}
	userIds := make([]int32, len(rawPlaylists))
	for idx, p := range rawPlaylists {
		userIds[idx] = p.PlaylistOwnerID
		for _, t := range p.PlaylistContents.TrackIDs {
			trackIds = append(trackIds, int32(t.Track))
		}
	}

	// fetch users + tracks in parallel
	g, ctx := errgroup.WithContext(ctx)
	userMap := map[int32]FullUser{}
	trackMap := map[int32]FullTrack{}

	// fetch users
	g.Go(func() error {
		users, err := q.FullUsers(ctx, GetUsersParams{
			MyID: arg.MyID,
			Ids:  userIds,
		})
		for _, user := range users {
			userMap[user.UserID] = user
		}
		return err
	})

	// fetch tracks
	g.Go(func() error {
		tracks, err := q.FullTracks(ctx, GetTracksParams{
			MyID: arg.MyID,
			Ids:  trackIds,
		})
		for _, track := range tracks {
			trackMap[track.TrackID] = track
		}
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
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

		var tracks = make([]FullTrack, 0, len(playlist.PlaylistContents.TrackIDs))
		for _, t := range playlist.PlaylistContents.TrackIDs {
			if track, ok := trackMap[int32(t.Track)]; ok {
				tracks = append(tracks, track)
			}
		}

		fullPlaylists = append(fullPlaylists, FullPlaylist{
			GetPlaylistsRow: playlist,
			ID:              id,
			// Artwork:         squareImageStruct(track.CoverArtSizes, track.CoverArt),
			User:   user,
			UserID: user.ID,
			Tracks: tracks,
		})
	}

	return fullPlaylists, nil
}

func ToMinPlaylist(fullPlaylist FullPlaylist) MinPlaylist {
	minTracks := make([]MinTrack, len(fullPlaylist.Tracks))
	for i, track := range fullPlaylist.Tracks {
		minTracks[i] = ToMinTrack(track)
	}

	return MinPlaylist{
		ID:               fullPlaylist.ID,
		PlaylistName:     fullPlaylist.PlaylistName,
		PlaylistOwnerID:  fullPlaylist.PlaylistOwnerID,
		PlaylistID:       fullPlaylist.PlaylistID,
		Artwork:          fullPlaylist.Artwork,
		PlaylistContents: fullPlaylist.PlaylistContents,
		Description:      nil,
		IsAlbum:          false,
		IsPrivate:        false,
		FavoriteCount:    0,
		RepostCount:      0,
		UserID:           fullPlaylist.UserID,
		User:             ToMinUser(fullPlaylist.User),
		Tracks:           minTracks,
	}
}

func ToMinPlaylists(fullPlaylists []FullPlaylist) []MinPlaylist {
	result := make([]MinPlaylist, len(fullPlaylists))
	for i, playlist := range fullPlaylists {
		result[i] = ToMinPlaylist(playlist)
	}
	return result
}
