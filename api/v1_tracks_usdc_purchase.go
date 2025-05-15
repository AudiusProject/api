package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1TracksUsdcPurchase(c *fiber.Ctx) error {
	myId := app.getMyId(c)

	limit := c.QueryInt("limit", 100)
	offset := c.QueryInt("offset", 0)
	timeRange := c.Query("time", "week")

	sql := `
		WITH usdc_track_ids AS MATERIALIZED (
			SELECT track_id
			FROM tracks
			WHERE
				is_unlisted = false AND
				is_available = true AND
				is_delete = false AND
				stream_conditions ? 'usdc_purchase'
			)
	    SELECT track_trending_scores.track_id
		FROM track_trending_scores
		JOIN usdc_track_ids ON track_trending_scores.track_id = usdc_track_ids.track_id
		WHERE type = 'TRACKS'
			AND version = 'pnagD'
			AND time_range = @time
		ORDER BY
			track_trending_scores.score DESC,
			track_trending_scores.track_id DESC
		LIMIT @limit
		OFFSET @offset
		`
	args := pgx.NamedArgs{}
	args["limit"] = limit
	args["offset"] = offset
	args["time"] = timeRange

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
