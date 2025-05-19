package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

type GenreMetric struct {
	Genre string `json:"genre"`
	Count int64  `json:"count"`
}

func (app *ApiServer) v1MetricsGenres(c *fiber.Ctx) error {
	limit := c.QueryInt("limit", 100)
	offset := c.QueryInt("offset", 0)
	startTime := time.Unix(int64(c.QueryInt("start_time", 0)), 0)

	metrics, err := app.queries.GetGenres(c.Context(), dbv1.GetGenresParams{
		LimitVal:  int32(limit),
		OffsetVal: int32(offset),
		StartTime: startTime,
	})
	if err != nil {
		return err
	}

	result := make([]GenreMetric, len(metrics))
	for i, metric := range metrics {
		result[i] = GenreMetric{
			Genre: string(metric.Genre.String),
			Count: metric.Count,
		}
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
