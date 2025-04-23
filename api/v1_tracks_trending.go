package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1TracksTrending(c *fiber.Ctx) error {
	myId := app.getMyId(c)

	sql := `
	SELECT track_trending_scores.track_id
	FROM track_trending_scores
	LEFT JOIN tracks
		ON tracks.track_id = track_trending_scores.track_id
		AND tracks.is_delete = false
		AND tracks.is_unlisted = false
		AND tracks.is_available = true
	WHERE type = 'TRACKS'
		AND version = 'pnagD'
		AND time_range = @time
		AND (@genre = '' OR track_trending_scores.genre = @genre)
	ORDER BY
		score DESC,
		track_id DESC
	LIMIT @limit
	OFFSET @offset
	`

	args := pgx.NamedArgs{}
	args["limit"] = c.Query("limit", "100")
	args["offset"] = c.Query("offset", "0")
	args["time"] = c.Query("time", "week")
	args["genre"] = c.Query("genre", "")

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	trackIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.GetTracksParams{
		Ids:  trackIds,
		MyID: myId,
	})

	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}
