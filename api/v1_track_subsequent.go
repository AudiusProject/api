package api

import (
	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type TrackSubsequentQueryParams struct {
	Limit int `query:"limit" default:"10" validate:"min=1,max=500"`
}

func (app *ApiServer) v1TrackSubsequent(c *fiber.Ctx) error {
	trackIdString := c.Params("trackId")
	trackId, err := trashid.DecodeHashId(trackIdString)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid track ID")
	}

	params := TrackSubsequentQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	sql := `
	WITH current_track AS (
		SELECT track_id, created_at
		FROM tracks
		WHERE track_id = @trackId
		AND is_current = true
		LIMIT 1
	),
    same_block_tracks AS (
        SELECT track_id, created_at
        FROM tracks
        WHERE is_current = true AND created_at = (SELECT created_at from current_track)
		AND track_id > @trackId
        ORDER BY track_id ASC
        LIMIT @limit
    ),
    future_tracks AS (
        SELECT track_id, created_at
        FROM tracks
        WHERE is_current = true AND created_at > (SELECT created_at from current_track)
        ORDER by created_at ASC, track_ID ASC
        LIMIT @limit
    ),
	subsequent_tracks AS (
		SELECT track_id, created_at
		FROM same_block_tracks
		UNION SELECT track_id, created_at FROM future_tracks
		ORDER BY created_at ASC, track_id ASC
		LIMIT @limit
	)
	SELECT track_id FROM subsequent_tracks
	`
	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"trackId": trackId,
		"limit":   params.Limit,
	})
	if err != nil {
		return err
	}

	tracks, err := pgx.CollectRows(rows, pgx.RowTo[trashid.HashId])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": tracks,
	})
}
