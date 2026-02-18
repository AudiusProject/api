package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetTrendingWinnersTracksParams struct {
	Week string `query:"week" default:""`
}

func (app *ApiServer) v1TracksTrendingWinners(c *fiber.Ctx) error {
	var params = GetTrendingWinnersTracksParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)

	trackIds, err := app.getTrendingWinnersIds(c, params.Week)
	if err != nil {
		return err
	}

	if len(trackIds) == 0 {
		return v1TracksResponse(c, []dbv1.Track{})
	}

	tracks, err := app.queries.Tracks(c.Context(), dbv1.TracksParams{
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

func (app *ApiServer) getTrendingWinnersIds(c *fiber.Ctx, weekParam string) ([]int32, error) {
	args := pgx.NamedArgs{
		"type": "TrendingType.TRACKS",
	}

	var weekFilter string
	if weekParam != "" {
		args["week"] = weekParam
		weekFilter = "AND tr.week = @week"
	} else {
		// Default: use the most recent week in the table that is before today
		weekFilter = `AND tr.week = (
			SELECT MAX(tr2.week)
			FROM trending_results tr2
			WHERE tr2.type = @type
				AND tr2.week < CURRENT_DATE
		)`
	}

	sql := `
		SELECT tr.id::int
		FROM trending_results tr
		JOIN tracks t ON t.track_id = tr.id::int
			AND t.is_current = true
			AND t.is_delete = false
			AND t.is_unlisted = false
			AND t.is_available = true
		WHERE tr.type = @type
			` + weekFilter + `
		ORDER BY tr.rank ASC
	`

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	trackIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return nil, err
	}

	return trackIds, nil
}
