package api

import (
	"net/http"
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetTrendingPlaylistsParams struct {
	Limit      int    `query:"limit" default:"30" validate:"min=1,max=100"`
	Offset     int    `query:"offset" default:"0" validate:"min=0"`
	Time       string `query:"time" default:"week" validate:"oneof=week month year"`
	Type       string `query:"type" default:"playlist" validate:"oneof=playlist album"`
	OmitTracks bool   `query:"omit_tracks" default:"false"`
}

func (app *ApiServer) v1PlaylistsTrending(c *fiber.Ctx) error {
	var params = GetTrendingPlaylistsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)
	filters := []string{
		"is_private = false",
		"is_delete = false",
		"is_current = true",
	}
	if params.Type == "album" {
		filters = append(filters, "is_album = true")
	} else {
		filters = append(filters, "is_album = false")
	}

	having := []string{}
	if params.Type == "album" {
		having = append(having, "COUNT(track_id) >= 1")
	} else {
		having = append(having, "COUNT(track_id) >= 5")
		having = append(having, "COUNT(DISTINCT owner_id) >= 5")
	}

	sql := `
		WITH qualified_playlists AS MATERIALIZED (
			WITH valid_playlists AS (
				SELECT playlist_id
				FROM playlists
				WHERE ` + strings.Join(filters, " AND ") + `
			),
			playlist_content AS (
				SELECT
					pt.playlist_id,
					t.owner_id,
					t.track_id
				FROM playlist_tracks pt
				JOIN valid_playlists p ON pt.playlist_id = p.playlist_id
				JOIN tracks t ON t.track_id = pt.track_id
				WHERE
					pt.is_removed = false
					AND t.is_delete = false
					AND t.is_current = true
			)
			SELECT
				playlist_id
			FROM
				playlist_content
			GROUP BY
				playlist_id
			HAVING
				` + strings.Join(having, " AND ") + `
		),
		filtered_scores AS MATERIALIZED (
			SELECT
				playlist_id,
				score
			FROM
				playlist_trending_scores
			WHERE
				type = 'PLAYLISTS'
				AND version = 'pnagD'
				AND time_range = @time
		)
		SELECT
			fs.playlist_id
		FROM qualified_playlists qp
		JOIN filtered_scores fs ON fs.playlist_id = qp.playlist_id
		ORDER BY fs.score DESC, fs.playlist_id DESC
		LIMIT @limit
		OFFSET @offset;
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
		OmitTracks: params.OmitTracks,
		// Limit these to 5 items to prevent slow load times
		TrackLimit: 5,
	})
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"data": playlists,
	})
}
