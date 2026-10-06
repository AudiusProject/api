package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetLatestTracksParams struct {
	Limit  int    `query:"limit" default:"100" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0,max=200"`
	Genre  string `query:"genre" default:""`
}

func (app *ApiServer) v1TracksLatest(c *fiber.Ctx) error {
	var params GetLatestTracksParams
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)

	trackIds, err := app.getLatestTrackIds(c, params.Genre, params.Limit, params.Offset)
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

func (app *ApiServer) getLatestTrackIds(c *fiber.Ctx, genre string, limit int, offset int) ([]int32, error) {
	sql := `
		SELECT t.track_id
		FROM tracks t
		JOIN users u ON u.user_id = t.owner_id
		WHERE t.is_current = true
			AND t.is_delete = false
			AND t.is_unlisted = false
			AND t.is_available = true
			AND u.is_deactivated = false
			AND u.is_available = true
			AND (@genre = '' OR t.genre = @genre)
		ORDER BY
			t.created_at DESC,
			t.track_id DESC
		LIMIT @limit
		OFFSET @offset
		`

	args := pgx.NamedArgs{
		"genre":  genre,
		"limit":  limit,
		"offset": offset,
	}

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(rows, pgx.RowTo[int32])
}
