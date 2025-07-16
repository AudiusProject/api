package api

import (
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetFeelingLuckyParams struct {
	Limit        int `query:"limit" default:"10" validate:"min=1,max=100"`
	MinFollowers int `query:"min_followers" validate:"omitempty,min=1"`
}

func (app *ApiServer) v1TracksFeelingLucky(c *fiber.Ctx) error {
	var params GetFeelingLuckyParams
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	// Conditionally join aggregate_user if min_followers is provided
	// to keep the query simple if we don't use follower filtering.
	joins := []string{
		"JOIN aggregate_plays ON aggregate_plays.play_item_id = tracks.track_id",
	}

	filters := []string{
		"is_current = true",
		"is_available = true",
		"is_delete = false",
		"is_unlisted = false",
		"stem_of IS NULL",
		"aggregate_plays.count >= 250",
	}
	if params.MinFollowers != 0 {
		joins = append(joins, "JOIN aggregate_user ON aggregate_user.user_id = tracks.owner_id")
		filters = append(filters, "aggregate_user.follower_count >= @minFollowers")
	}

	sql := `
		WITH filtered AS (
			SELECT track_id from tracks
			` + strings.Join(joins, "\n") + `
			WHERE ` + strings.Join(filters, " AND ") + `
		) SELECT * from filtered
		ORDER BY random()
		LIMIT @limit;
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"limit":        params.Limit,
		"minFollowers": params.MinFollowers,
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
			MyID: app.getMyId(c),
		},
	})

	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}
