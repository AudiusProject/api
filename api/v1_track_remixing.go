package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetTrackRemixingParams struct {
	Limit  int `query:"limit" default:"20" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

func (app *ApiServer) v1TrackRemixing(c *fiber.Ctx) error {
	params := GetTrackRemixingParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)
	trackId := c.Locals("trackId").(int)
	sql := `
		SELECT pt.track_id
		FROM tracks pt
		JOIN remixes r ON r.parent_track_id = pt.track_id AND r.child_track_id = @trackId
		JOIN tracks ct ON ct.track_id = @trackId
		WHERE pt.is_current = true
		AND pt.is_unlisted = false
		AND ct.is_current = true
		AND ct.is_stream_gated = false
		ORDER BY pt.created_at DESC, pt.track_id DESC
		LIMIT @limit OFFSET @offset
	`

	// Get all remix tracks in a single query
	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"trackId": trackId,
		"limit":   params.Limit,
		"offset":  params.Offset,
	})

	if err != nil {
		return err
	}

	trackIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:  trackIds,
			MyID: myId,
		},
	})

	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}
