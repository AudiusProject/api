package api

import (
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type RemixedTrackAggregate struct {
	TrackID    trashid.HashId `json:"track_id"`
	Title      string         `json:"title"`
	RemixCount int            `json:"remix_count"`
}

type GetUsersTracksRemixedParams struct {
	Limit  int `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

func (app *ApiServer) v1UserTracksRemixed(c *fiber.Ctx) error {
	params := GetUsersTracksRemixedParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}
	userId := app.getUserId(c)

	sql := `
		SELECT
			parent_track_id as track_id,
			tracks.title,
			count(remixes.child_track_id) as remix_count
		FROM remixes
		JOIN tracks ON tracks.track_id = remixes.parent_track_id
		WHERE tracks.owner_id = @userId
		GROUP BY remixes.parent_track_id, tracks.title
		ORDER BY remix_count desc
		LIMIT @limit
		OFFSET @offset;
	`

	args := pgx.NamedArgs{
		"userId": userId,
		"limit":  params.Limit,
		"offset": params.Offset,
	}

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	remixes, err := pgx.CollectRows(rows, pgx.RowToStructByName[RemixedTrackAggregate])

	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": remixes,
	})
}
