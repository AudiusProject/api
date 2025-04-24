package api

import (
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) getTrendingIds(c *fiber.Ctx, timeRange string, genre string, limit int, offset int) ([]int32, error) {
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
	args["limit"] = limit
	args["offset"] = offset
	args["time"] = timeRange
	args["genre"] = genre

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return nil, err
	}

	trackIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return nil, err
	}

	return trackIds, nil
}

type hashIdResponse struct {
	ID string `json:"id"`
}

func encodeIds(ids []int32) ([]hashIdResponse, error) {
	result := make([]hashIdResponse, len(ids))
	for i, id := range ids {
		encoded, err := trashid.EncodeHashId(int(id))
		if err != nil {
			return nil, err
		}
		result[i] = hashIdResponse{ID: encoded}
	}
	return result, nil
}

func (app *ApiServer) v1TracksTrendingIds(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 100)
	offset := c.QueryInt("offset", 0)
	genre := ""

	weekChan := make(chan []int32)
	monthChan := make(chan []int32)
	yearChan := make(chan []int32)
	errChan := make(chan error)

	go func() {
		ids, err := app.getTrendingIds(c, "week", genre, limit, offset)
		if err != nil {
			errChan <- err
			return
		}
		weekChan <- ids
	}()

	go func() {
		ids, err := app.getTrendingIds(c, "month", genre, limit, offset)
		if err != nil {
			errChan <- err
			return
		}
		monthChan <- ids
	}()

	go func() {
		ids, err := app.getTrendingIds(c, "allTime", genre, limit, offset)
		if err != nil {
			errChan <- err
			return
		}
		yearChan <- ids
	}()

	var weekIds, monthIds, yearIds []int32

	for i := 0; i < 3; i++ {
		select {
		case weekIds = <-weekChan:
		case monthIds = <-monthChan:
		case yearIds = <-yearChan:
		case err := <-errChan:
			return err
		}
	}

	weekHashedIds, err := encodeIds(weekIds)
	if err != nil {
		return err
	}

	monthHashedIds, err := encodeIds(monthIds)
	if err != nil {
		return err
	}

	yearHashedIds, err := encodeIds(yearIds)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"week":  weekHashedIds,
			"month": monthHashedIds,
			// Note that this is technically all time, but is set as
			// year for backwards compatibility
			"year": yearHashedIds,
		},
	})
}
