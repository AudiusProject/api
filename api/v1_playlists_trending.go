package api

import (
	"net/http"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetTrendingPlaylistsParams struct {
	Limit  int    `query:"limit" default:"30" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0"`
	Time   string `query:"time" default:"week" validate:"oneof=week month year"`
}

func (app *ApiServer) v1PlaylistsTrending(c *fiber.Ctx) error {
	var params = GetTrendingPlaylistsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)

	sql := `
		SELECT playlist_trending_scores.playlist_id
		FROM playlist_trending_scores
		JOIN playlists
			ON playlists.playlist_id = playlist_trending_scores.playlist_id
			AND playlists.is_current = true
			AND playlists.is_delete = false
			AND playlists.is_private = false
			AND playlists.is_album = false
		WHERE type = 'PLAYLISTS'
			AND version = 'pnagD'
			AND time_range = @time
		ORDER BY
			score DESC,
			playlist_id DESC
		LIMIT @limit
		OFFSET @offset
		`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"limit":  params.Limit,
		"offset": params.Offset,
		"time":   params.Time,
	})
	if err != nil {
		return err
	}

	ids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	playlists, err := app.queries.FullPlaylists(c.Context(), dbv1.FullPlaylistsParams{
		GetPlaylistsParams: dbv1.GetPlaylistsParams{
			Ids:  ids,
			MyID: myId,
		},
	})

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"data": playlists,
	})
}
