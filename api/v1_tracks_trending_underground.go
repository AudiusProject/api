package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1TracksTrendingUnderground(c *fiber.Ctx) error {
	myId := app.getMyId(c)

	limit := c.QueryInt("limit", 100)
	offset := c.QueryInt("offset", 0)
	timeRange := c.Query("time", "week")
	genre := c.Query("genre", "")

	sql := `
		SELECT track_trending_scores.track_id
		FROM track_trending_scores
		LEFT JOIN tracks
			ON tracks.track_id = track_trending_scores.track_id
			AND tracks.is_delete = false
			AND tracks.is_unlisted = false
			AND tracks.is_available = true
		LEFT JOIN aggregate_user
			ON aggregate_user.user_id = tracks.owner_id
		WHERE type = 'TRACKS'
			AND version = 'pnagD'
			AND time_range = @time
			AND (@genre = '' OR track_trending_scores.genre = @genre)
			AND aggregate_user.follower_count < 1500
			AND aggregate_user.following_count < 1500
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
	args["genre"] = genre

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
