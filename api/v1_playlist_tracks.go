package api

import (
	"strings"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type PlaylistTracksParams struct {
	ExcludeGated bool `query:"exclude_gated" default:"true" validate:"boolean"`
}

func (app *ApiServer) v1PlaylistTracks(c *fiber.Ctx) error {
	var params PlaylistTracksParams
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	excludeGated := c.QueryBool("exclude_gated", true)
	myId := app.getMyId(c)
	playlistId, err := trashid.DecodeHashId(c.Params("playlistId"))
	if err != nil {
		return err
	}

	filters := []string{
		"p.playlist_id = @playlistId",
		"t.is_delete = false",
		"t.is_current = true",
	}
	if excludeGated {
		filters = append(filters, "t.is_stream_gated != true")
	}
	filterString := strings.Join(filters, " AND ")

	sql := `
	SELECT
		(e.value->>'track')::int AS track_id
	FROM playlists p
	CROSS JOIN LATERAL jsonb_array_elements(p.playlist_contents->'track_ids') AS e(value)
	JOIN tracks t
	ON t.track_id = (e.value->>'track')::int
	WHERE ` + filterString + `;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"playlistId": playlistId,
	})
	if err != nil {
		return err
	}
	trackIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	tracks, err := app.queries.Tracks(c.Context(), dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:             trackIds,
			MyID:            myId,
			AuthedWallet:    app.tryGetAuthedWallet(c),
			IncludeUnlisted: true,
		},
	})

	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}
