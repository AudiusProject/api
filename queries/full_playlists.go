package queries

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
