package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

const minNewAlbumValidationScore = 5

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
	joinClause := ""
	filterClause := ""
	orderClause := "COALESCE(p.release_date, p.created_at) DESC, p.playlist_id DESC"

	if isAlbum {
		joinClause = "LEFT JOIN aggregate_playlist ap ON p.playlist_id = ap.playlist_id"
		filterClause = `
		  AND COALESCE(ap.save_count, 0) + COALESCE(ap.repost_count, 0) >= @min_validation_score
		`
		orderClause = `
		  COALESCE(ap.save_count, 0) + COALESCE(ap.repost_count, 0) DESC,
		  COALESCE(p.release_date, p.created_at) DESC,
		  p.playlist_id DESC
		`
	}

	sql := `
		SELECT p.playlist_id
		FROM playlists p
		` + joinClause + `
		WHERE p.is_delete = false
		  AND p.is_current = true
		  AND p.is_private = false
		  AND p.is_album = @is_album
		  AND COALESCE(p.release_date, p.created_at) <= NOW()
		  AND COALESCE(p.release_date, p.created_at) > NOW() - INTERVAL '90 days'
		  ` + filterClause + `
		ORDER BY ` + orderClause + `
		LIMIT @limit
		OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"is_album":             isAlbum,
		"limit":                params.Limit,
		"offset":               params.Offset,
		"min_validation_score": minNewAlbumValidationScore,
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
