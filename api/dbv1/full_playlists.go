package dbv1

import (
	"context"

	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5/pgtype"
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

func (q *Queries) FullPlaylistsKeyed(ctx context.Context, arg GetPlaylistsParams) (map[int32]FullPlaylist, error) {
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
		var err error
		userMap, err = q.FullUsersKeyed(ctx, GetUsersParams{
			MyID: arg.MyID,
			Ids:  userIds,
		})
		return err
	})

	// fetch tracks
	g.Go(func() error {
		var err error
		trackMap, err = q.FullTracksKeyed(ctx, GetTracksParams{
			MyID: arg.MyID,
			Ids:  trackIds,
		})
		return err
	})

	if err := g.Wait(); err != nil {
		return nil, err
	}

	playlistMap := map[int32]FullPlaylist{}
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

		playlistMap[playlist.PlaylistID] = FullPlaylist{
			GetPlaylistsRow: playlist,
			ID:              id,
			// Artwork:         squareImageStruct(track.CoverArtSizes, track.CoverArt),
			User:   user,
			UserID: user.ID,
			Tracks: tracks,
		}
	}

	return playlistMap, nil
}

func (q *Queries) FullPlaylists(ctx context.Context, arg GetPlaylistsParams) ([]FullPlaylist, error) {
	playlistMap, err := q.FullPlaylistsKeyed(ctx, arg)
	if err != nil {
		return nil, err
	}

	// return in same order as input list of ids
	// some ids may be not found...
	fullPlaylists := []FullPlaylist{}
	for _, id := range arg.Ids {
		if t, found := playlistMap[id]; found {
			fullPlaylists = append(fullPlaylists, t)
		}
	}

	return fullPlaylists, nil
}

type MinPlaylist struct {
	ID               string      `json:"id"`
	PlaylistName     pgtype.Text `json:"playlist_name"`
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
