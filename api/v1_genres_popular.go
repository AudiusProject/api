package api

import (
	"context"
	"fmt"
	"sort"
	"time"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
)

// genresPopularCacheTTL bounds how stale the popular-genre ranking may be.
// The endpoint runs a GROUP BY genre scan over the tracks table plus an
// in-process normalize/merge/sort, and the result is an approximate popularity
// ranking, so a short TTL is fine.
const genresPopularCacheTTL = 15 * time.Minute

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

	genres, err := app.getPopularGenres(c.Context(), params.Limit, params.Offset, startTime)
	if err != nil {
		return err
	}

	// min_count is applied per request rather than baked into the cache key, so
	// a single cached (limit, offset, startTime bucket) slice serves every
	// threshold. Build a fresh slice — `genres` is the shared cached value and
	// must not be mutated/aliased.
	filtered := make([]PopularGenre, 0, len(genres))
	for _, g := range genres {
		if g.Count < int64(params.MinCount) {
			continue
		}
		filtered = append(filtered, g)
	}

	return c.JSON(fiber.Map{
		"data": filtered,
	})
}

// getPopularGenres returns the normalized, merged, and sorted popular-genre
// slice for the given query window, caching it for genresPopularCacheTTL. The
// min_count filter is intentionally NOT applied here so the same cached slice
// serves every threshold. startTime is a rolling window (now minus a
// client-supplied offset), so it's bucketed to the TTL to keep the cache key
// stable within each window; the DB query still uses the exact startTime on a
// miss. The returned slice is shared across callers and must not be mutated.
func (app *ApiServer) getPopularGenres(ctx context.Context, limit, offset int, startTime time.Time) ([]PopularGenre, error) {
	bucket := startTime.Truncate(genresPopularCacheTTL).Unix()
	cacheKey := fmt.Sprintf("%d:%d:%d", limit, offset, bucket)
	if hit, ok := app.genresPopularCache.Get(cacheKey); ok {
		return hit, nil
	}

	genres, err := app.queries.GetGenres(ctx, dbv1.GetGenresParams{
		LimitVal:  int32(limit),
		OffsetVal: int32(offset),
		StartTime: startTime,
	})
	if err != nil {
		return nil, err
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

	// Re-sort because merging variants can change the relative ordering.
	sort.SliceStable(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	app.genresPopularCache.Set(cacheKey, result)
	return result, nil
}
