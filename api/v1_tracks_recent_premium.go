package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetTracksRecentPremiumParams struct {
	Limit  int `query:"limit" default:"100" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

func (app *ApiServer) v1TracksRecentPremium(c *fiber.Ctx) error {
	myId := app.getMyId(c)

	var params = GetTracksRecentPremiumParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	// Note: limiting CTE to past month to speed this up.
	sql := `
		WITH most_recent_tracks AS (
			SELECT DISTINCT ON (owner_id) track_id
			FROM tracks
			WHERE
				is_unlisted = false AND
				is_available = true AND
				is_delete = false AND
				(stream_conditions ? 'usdc_purchase' OR download_conditions ? 'usdc_purchase') AND
				created_at >= now() - interval '1 month'
			ORDER BY owner_id, created_at DESC
		)
		SELECT track_id
		FROM most_recent_tracks
		LIMIT @limit
		OFFSET @offset;
		`
	args := pgx.NamedArgs{}
	args["limit"] = params.Limit
	args["offset"] = params.Offset

	rows, err := app.pool.Query(c.Context(), sql, args)
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
