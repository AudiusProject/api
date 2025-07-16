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

	sql := `
		WITH played_tracks AS (
			SELECT DISTINCT play_item_id
			FROM plays WHERE user_id=@userId
		),
		top_genres AS (
			SELECT tracks.genre
			FROM played_tracks
			JOIN tracks ON tracks.track_id = played_tracks.play_item_id
			WHERE genre IS NOT NULL
			GROUP BY tracks.genre
			ORDER BY count(*) DESC
			LIMIT 4
		),
		max_scores AS (
			SELECT track_id, genre, score, time_range
			FROM track_trending_scores
			WHERE genre IN (SELECT genre FROM top_genres)
			AND time_range = @timeRange
		),
		ranked_tracks AS (
		SELECT
			tracks.track_id,
			max_scores.genre,
			max_scores.score,
			row_number() over (
			PARTITION BY max_scores.genre
			ORDER BY random()
			) AS rank
		FROM max_scores
		JOIN tracks ON tracks.track_id = max_scores.track_id
		LEFT JOIN played_tracks ON played_tracks.play_item_id = max_scores.track_id
		WHERE played_tracks.play_item_id IS NULL
			AND tracks.is_unlisted = false
			AND tracks.is_current = true
			AND tracks.is_delete = false
			AND score >
			CASE
				WHEN max_scores.time_range = 'allTime' THEN 10000000000
				WHEN max_scores.time_range = 'week' THEN 10000
				ELSE 1000000
				END
		)
		SELECT track_id
		FROM ranked_tracks
		WHERE rank <= 3
		ORDER BY score DESC
		LIMIT @limit
		OFFSET @offset;
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
