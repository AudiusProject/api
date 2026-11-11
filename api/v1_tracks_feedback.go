package api

import (
	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetFeedbackTracksParams struct {
	Limit  int    `query:"limit" default:"20" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0,max=500"`
	Genre  string `query:"genre" default:""`
}

// v1TracksFeedback lists tracks whose artist asked for feedback, newest
// request first.
func (app *ApiServer) v1TracksFeedback(c *fiber.Ctx) error {
	var params GetFeedbackTracksParams
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	sql := `
		SELECT t.track_id
		FROM tracks t
		JOIN users u ON u.user_id = t.owner_id
		WHERE t.feedback_requested_at IS NOT NULL
			AND t.is_current = true
			AND t.is_delete = false
			AND t.is_unlisted = false
			AND t.is_available = true
			AND t.is_stream_gated = false
			AND t.access_authorities IS NULL
			AND COALESCE(t.comments_disabled, false) = false
			AND u.is_deactivated = false
			AND u.is_available = true
			AND (@genre = '' OR t.genre = @genre)
		ORDER BY
			t.feedback_requested_at DESC,
			t.track_id DESC
		LIMIT @limit
		OFFSET @offset
	`
	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"genre":  params.Genre,
		"limit":  params.Limit,
		"offset": params.Offset,
	})
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
			MyID:         app.getMyId(c),
			AuthedWallet: app.tryGetAuthedWallet(c),
		},
	})
	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}
