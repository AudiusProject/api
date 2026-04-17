package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetPlaylistsNewReleasesParams struct {
	Limit  int    `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0"`
	Type   string `query:"type" default:"playlist" validate:"oneof=playlist album"`
}

func (app *ApiServer) v1PlaylistsNewReleases(c *fiber.Ctx) error {
	params := GetPlaylistsNewReleasesParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	isAlbum := params.Type == "album"

	sql := `
		SELECT playlist_id
		FROM playlists
		WHERE is_delete = false
		  AND is_current = true
		  AND is_private = false
		  AND is_album = @is_album
		  AND COALESCE(release_date, created_at) <= NOW()
		  AND COALESCE(release_date, created_at) > NOW() - INTERVAL '90 days'
		ORDER BY COALESCE(release_date, created_at) DESC, playlist_id DESC
		LIMIT @limit
		OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"is_album": isAlbum,
		"limit":    params.Limit,
		"offset":   params.Offset,
	})
	if err != nil {
		return err
	}

	ids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	playlists, err := app.queries.Playlists(c.Context(), dbv1.PlaylistsParams{
		GetPlaylistsParams: dbv1.GetPlaylistsParams{
			Ids:  ids,
			MyID: app.getMyId(c),
		},
		OmitTracks:   true,
		AuthedWallet: app.tryGetAuthedWallet(c),
	})
	if err != nil {
		return err
	}

	return v1PlaylistsResponse(c, playlists)
}
