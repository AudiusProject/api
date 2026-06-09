package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetTracksUsdcPurchaseParams struct {
	Limit  int    `query:"limit" default:"100" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0,max=200"`
	Time   string `query:"time" default:"week" validate:"oneof=week month allTime"`
}

func (app *ApiServer) v1TracksUsdcPurchase(c *fiber.Ctx) error {
	myId := app.getMyId(c)

	var params = GetTracksUsdcPurchaseParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	sql := `
		SELECT track_trending_scores.track_id
		FROM track_trending_scores
		WHERE type = 'TRACKS'
			AND version = 'pnagD'
			AND time_range = @time
			AND EXISTS (
				SELECT 1
				FROM tracks
				WHERE tracks.track_id = track_trending_scores.track_id
					AND tracks.is_unlisted = false
					AND tracks.is_available = true
					AND tracks.is_delete = false
					AND tracks.stream_conditions ? 'usdc_purchase'
			)
		ORDER BY
			track_trending_scores.score DESC,
			track_trending_scores.track_id DESC
		LIMIT @limit
		OFFSET @offset
		`
	args := pgx.NamedArgs{}
	args["limit"] = params.Limit
	args["offset"] = params.Offset
	args["time"] = params.Time

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	trackIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	tracks, err := app.queries.Tracks(c.Context(), dbv1.TracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:          trackIds,
			MyID:         myId,
			AuthedWallet: app.tryGetAuthedWallet(c),
		},
	})

	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}
