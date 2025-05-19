package dbv1

import (
	"context"
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5"
)

type FullAccountPlaylistOwner struct {
	ID            trashid.HashId `json:"id"`
	Handle        string         `json:"handle"`
	IsDeactivated bool           `json:"is_deactivated"`
}

type FullAccountPlaylist struct {
	ID         trashid.HashId `json:"id"`
	PlaylistID int32          `json:"playlist_id"`
	Name       string         `json:"name"`
	IsAlbum    bool           `json:"is_album"`
	UserID     trashid.HashId `json:"user_id"`
	CreatedAt  time.Time      `json:"created_at"`
	Permalink  string         `json:"permalink"`

	User FullAccountPlaylistOwner `json:"user"`
}

func (q *Queries) FullAccountPlaylists(ctx context.Context, userID int32) ([]FullAccountPlaylist, error) {

	rows, err := q.db.Query(ctx, mustGetQuery("get_account_playlists.sql"), pgx.NamedArgs{
		"user_id": userID,
	})
	if err != nil {
		return nil, err
	}

	// todo: permalink

	return pgx.CollectRows(rows, pgx.RowToStructByNameLax[FullAccountPlaylist])
}
