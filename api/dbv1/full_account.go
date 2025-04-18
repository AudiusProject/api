package dbv1

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jackc/pgx/v5"
)

type FullAccount struct {
	User            FullUser              `json:"user"`
	Playlists       []FullAccountPlaylist `json:"playlists"`
	PlaylistLibrary PlaylistLibrary       `json:"playlist_library"`
	TrackSaveCount  int64                 `json:"track_save_count"`
}

func (q *Queries) FullAccount(ctx context.Context, wallet string) (*FullAccount, error) {
	// resolve wallet to user id
	userId, err := q.GetUserForWallet(ctx, wallet)

	if err != nil {
		return nil, err
	}

	users, err := q.FullUsers(ctx, GetUsersParams{
		Ids:  []int32{int32(userId)},
		MyID: userId,
	})
	if err != nil {
		return nil, err
	}

	if len(users) == 0 {
		return nil, pgx.ErrNoRows
	}

	playlists, err := q.FullAccountPlaylists(ctx, int32(userId))
	if err != nil {
		return nil, err
	}

	accountFields, err := q.GetExtendedAccountFields(ctx, userId)
	playlistLibrary := PlaylistLibrary{}
	err = json.Unmarshal(accountFields.PlaylistLibrary, &playlistLibrary)

	if err != nil {
		fmt.Printf("error unmarshalling playlist library: %+v\n", err)
		return nil, err
	}

	return &FullAccount{
		User:            users[0],
		Playlists:       playlists,
		PlaylistLibrary: playlistLibrary,
		TrackSaveCount:  accountFields.TrackSaveCount.Int64,
	}, nil
}
