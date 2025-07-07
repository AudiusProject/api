package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetTracksRecentCommentsParams struct {
	Limit  int `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

func (app *ApiServer) v1TracksRecentComments(c *fiber.Ctx) error {
	myId := app.getMyId(c)

	var params = GetTracksRecentPremiumParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	// Note: limiting CTE to past month to speed this up.
	// TODO: Could we go even shorter?
	sql := `
		SELECT c.entity_id
		FROM comments c
		JOIN tracks t ON t.track_id = c.entity_id
		WHERE
			c.entity_type = 'Track' AND
			t.is_unlisted = false AND
			t.is_available = true AND
			t.is_delete = false AND
			c.created_at >= now() - interval '1 month'
		GROUP BY c.entity_id
		ORDER BY MAX(c.created_at) DESC
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
