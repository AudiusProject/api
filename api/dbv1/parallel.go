package dbv1

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type ParallelParams struct {
	UserIds     []int32
	TrackIds    []int32
	PlaylistIds []int32
	MyID        interface{}
}

type ParallelResult struct {
	UserMap     map[int32]FullUser
	TrackMap    map[int32]FullTrack
	PlaylistMap map[int32]FullPlaylist
}

func (q *Queries) Parallel(ctx context.Context, arg ParallelParams) (*ParallelResult, error) {
	g, ctx := errgroup.WithContext(ctx)

	var userMap map[int32]FullUser
	var trackMap map[int32]FullTrack
	var playlistMap map[int32]FullPlaylist

	if len(arg.UserIds) > 0 {
		g.Go(func() error {
			var err error
			userMap, err = q.FullUsersKeyed(ctx, GetUsersParams{
				Ids:  arg.UserIds,
				MyID: arg.MyID,
			})
			return err
		})
	}

	if len(arg.TrackIds) > 0 {
		g.Go(func() error {
			var err error
			trackMap, err = q.FullTracksKeyed(ctx, GetTracksParams{
				Ids:  arg.TrackIds,
				MyID: arg.MyID,
			})
			return err
		})
	}

	if len(arg.PlaylistIds) > 0 {
		g.Go(func() error {
			var err error
			playlistMap, err = q.FullPlaylistsKeyed(ctx, GetPlaylistsParams{
				Ids:  arg.PlaylistIds,
				MyID: arg.MyID,
			})
			return err
		})
	}

	err := g.Wait()
	if err != nil {
		return nil, err
	}

	result := &ParallelResult{
		userMap,
		trackMap,
		playlistMap,
	}

	return result, nil
}
