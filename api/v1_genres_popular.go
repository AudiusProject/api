package api

import (
	"sort"
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

	// Genre values are written upstream (by the discovery provider) and are not
	// normalized at rest, so the same logical genre can appear under several
	// spellings ("Hip Hop", "hip-hop", "hiphop"). Collapse those variants to a
	// canonical name here and sum their counts. NOTE: the SQL groups + paginates
	// on the raw genre, so this only merges variants that land within the same
	// page; the durable fix is to normalize genre on write upstream.
	indexByName := make(map[string]int, len(genres))
	result := make([]PopularGenre, 0, len(genres))
	for _, genre := range genres {
		name := NormalizeGenre(genre.Genre.String)
		if name == "" {
			continue
		}

		if i, ok := indexByName[name]; ok {
			result[i].Count += genre.Count
			continue
		}

		indexByName[name] = len(result)
		result = append(result, PopularGenre{
			Name:  name,
			Count: genre.Count,
		})
	}

	// Re-sort because merging variants can change the relative ordering, then
	// drop anything below min_count (evaluated against the merged total).
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	filtered := result[:0]
	for _, g := range result {
		if g.Count < int64(params.MinCount) {
			continue
		}
		filtered = append(filtered, g)
	}

	return c.JSON(fiber.Map{
		"data": filtered,
	})
}
