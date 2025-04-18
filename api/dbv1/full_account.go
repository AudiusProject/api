package dbv1

import (
	"context"
	"encoding/json"

	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5"
)

type FullAccount struct {
	User            FullUser              `json:"user"`
	Playlists       []FullAccountPlaylist `json:"playlists"`
	PlaylistLibrary json.RawMessage       `json:"playlist_library"`
	TrackSaveCount  int64                 `json:"track_save_count"`
}

// Recursively process playlists in the library and hashify playlist_ids for
// non-explore playlists.
func processPlaylistLibraryItem(v any) any {
	switch val := v.(type) {
	case map[string]any:
		// Handle playlist_id if this is a playlist
		if playlistType, ok := val["type"].(string); ok && playlistType == "playlist" {
			if playlistID, ok := val["playlist_id"].(float64); ok {
				encodedID, _ := trashid.EncodeHashId(int(playlistID))
				val["playlist_id"] = encodedID
			}
		}

		// Process nested contents
		if contents, ok := val["contents"].([]any); ok {
			for i, item := range contents {
				contents[i] = processPlaylistLibraryItem(item)
			}
		}
		return val
	default:
		return v
	}
}

// TODO: tests
func processPlaylistLibrary(library []byte) ([]byte, error) {
	var playlistLibrary any
	if err := json.Unmarshal(library, &playlistLibrary); err != nil {
		return nil, err
	}

	processedLibrary := processPlaylistLibraryItem(playlistLibrary)
	return json.Marshal(processedLibrary)
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

	accountFields, err := q.GetExtendedAccountFields(ctx, userId)

	playlistLibrary, err := processPlaylistLibrary(accountFields.PlaylistLibrary)

	if err != nil {
		return nil, err
	}

	return &FullAccount{
		User:            users[0],
		Playlists:       playlists,
		PlaylistLibrary: playlistLibrary,
		TrackSaveCount:  accountFields.TrackSaveCount.Int64,
	}, nil
}
