package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

type GetMetricsGenresParams struct {
	StartTime int `query:"start_time" default:"0"`
	Limit     int `query:"limit" default:"100" validate:"min=1,max=100"`
	Offset    int `query:"offset" default:"0" validate:"min=0"`
}

type GenreMetric struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

func (app *ApiServer) v1MetricsGenres(c *fiber.Ctx) error {
	params := GetMetricsGenresParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	startTime := time.Unix(int64(params.StartTime), 0)

	metrics, err := app.queries.GetGenres(c.Context(), dbv1.GetGenresParams{
		LimitVal:  int32(params.Limit),
		OffsetVal: int32(params.Offset),
		StartTime: startTime,
	})
	if err != nil {
		return err
	}

	result := make([]GenreMetric, len(metrics))
	for i, metric := range metrics {
		result[i] = GenreMetric{
			Name:  string(metric.Genre.String),
			Count: metric.Count,
		}
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
