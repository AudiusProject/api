package api

import (
	"time"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

type GetPopularGenresParams struct {
	StartTime int `query:"start_time" default:"0"`
	Limit     int `query:"limit" default:"100" validate:"min=1,max=100"`
	Offset    int `query:"offset" default:"0" validate:"min=0"`
	MinCount  int `query:"min_count" default:"1" validate:"min=1,max=1000000"`
}

type PopularGenre struct {
	Name  string `json:"name"`
	Count int64  `json:"count"`
}

func (app *ApiServer) v1GenresPopular(c *fiber.Ctx) error {
	params := GetPopularGenresParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	startTime := time.Unix(int64(params.StartTime), 0)

	genres, err := app.queries.GetGenres(c.Context(), dbv1.GetGenresParams{
		LimitVal:  int32(params.Limit),
		OffsetVal: int32(params.Offset),
		StartTime: startTime,
	})
	if err != nil {
		return err
	}

	result := make([]PopularGenre, 0, len(genres))
	for _, genre := range genres {
		if genre.Count < int64(params.MinCount) {
			continue
		}

		result = append(result, PopularGenre{
			Name:  genre.Genre.String,
			Count: genre.Count,
		})
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
