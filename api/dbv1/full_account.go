package dbv1

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

type FullAccount struct {
	User            FullUser              `json:"user"`
	Playlists       []FullAccountPlaylist `json:"playlists"`
	PlaylistLibrary json.RawMessage       `json:"playlist_library"`
	TrackSaveCount  int64                 `json:"track_save_count"`
}

func (q *Queries) FullAccount(ctx context.Context, wallet string) (*FullAccount, error) {
	// resolve wallet to user id
	userId, err := q.GetUserForWallet(ctx, wallet)

	if err != nil {
		return nil, err
	}

	users, err := q.FullUsers(ctx, GetUsersParams{
		Ids:  []int32{userId},
		MyID: userId,
	})
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, pgx.ErrNoRows
	}

	playlists, err := q.FullAccountPlaylists(ctx, userId)
	if err != nil {
		return nil, err
	}

	// Extract playlist_library from user record
	playlistLibrary := users[0].PlaylistLibrary
	trackSaveCount := users[0].TrackSaveCount
	// Create a copy of the user without playlist_library/track_save_count as
	// they are deprecated fields and we will return them as siblings
	userWithoutLibrary := users[0]
	userWithoutLibrary.PlaylistLibrary = nil
	userWithoutLibrary.TrackSaveCount = nil

	return &FullAccount{
		User:            userWithoutLibrary,
		Playlists:       playlists,
		PlaylistLibrary: playlistLibrary,
		TrackSaveCount:  *trackSaveCount,
	}, nil
}
