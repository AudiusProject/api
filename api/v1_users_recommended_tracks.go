package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersRecommendedTracksParams struct {
	Limit     int    `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset    int    `query:"offset" default:"0" validate:"min=0"`
	TimeRange string `query:"time_range" default:"week" validate:"oneof=week month allTime"`
}

func (app *ApiServer) v1UsersRecommendedTracks(c *fiber.Ctx) error {
	params := GetUsersRecommendedTracksParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)

	var timeRange = "allTime"
	switch params.TimeRange {
	case "week":
		timeRange = "week"
	case "month":
		timeRange = "month"
	}

	// Find recommendations by looking at the top tracks from the user's top genres,
	// filtering out already played tracks and picking randomly from them.
	sql := `
		WITH played_tracks AS (
			SELECT DISTINCT play_item_id
			FROM plays
			WHERE user_id = @userId
		),
		top_genres AS (
			SELECT t.genre
			FROM played_tracks pt
			JOIN tracks t ON t.track_id = pt.play_item_id
			WHERE t.genre IS NOT NULL
			GROUP BY t.genre
			ORDER BY COUNT(*) DESC
			LIMIT 5
		),
		top_tracks_per_genre AS (
			SELECT
				tg.genre,
				tts.track_id,
				tts.score
			FROM top_genres tg
			JOIN LATERAL (
				SELECT tts.track_id, tts.score
				FROM (
					SELECT tts.track_id, tts.score
					FROM track_trending_scores tts
					WHERE tts.genre = tg.genre
						AND tts.time_range = @timeRange
						AND tts.version = 'pnagD'
					ORDER BY tts.score DESC
					-- Limit the number of tracks we recall to improve performance
					LIMIT 3000
				) tts
				WHERE NOT EXISTS (
					SELECT 1 FROM played_tracks pt WHERE pt.play_item_id = tts.track_id
				)
			LIMIT 10
			) tts ON true
		)
		SELECT t.track_id
		FROM top_tracks_per_genre
		JOIN tracks t ON top_tracks_per_genre.track_id = t.track_id
		WHERE
			t.is_unlisted = false
			AND t.is_current = true
			AND t.is_delete = false
		ORDER BY random()
		LIMIT 10
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"limit":     params.Limit,
		"offset":    params.Offset,
		"timeRange": timeRange,
		"userId":    app.getUserId(c),
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
			MyID: myId,
		},
	})

	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)

}
