package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/maypok86/otter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGetPopularGenresCache exercises the cache layer of getPopularGenres
// without a DB: with no pool wired, any cache miss would nil-panic on
// app.queries, so every assertion here that returns successfully proves the
// value came from the cache. It also pins the (limit, offset, startTime bucket)
// key contract.
func TestGetPopularGenresCache(t *testing.T) {
	cache, err := otter.MustBuilder[string, []PopularGenre](16).
		WithTTL(genresPopularCacheTTL).
		Build()
	require.NoError(t, err)
	app := &ApiServer{genresPopularCache: &cache}

	// Aligned to a 15m boundary (1_700_000_100 = 1888889 * 900) so a small
	// positive delta stays inside the same bucket.
	startTime := time.Unix(1_700_000_100, 0)
	seeded := []PopularGenre{{Name: "Electronic", Count: 42}, {Name: "Hip-Hop", Count: 17}}

	// Seed under the exact key getPopularGenres computes for (100, 0, startTime).
	bucket := startTime.Truncate(genresPopularCacheTTL).Unix()
	cache.Set(fmt.Sprintf("%d:%d:%d", 100, 0, bucket), seeded)

	// A request later in the same TTL bucket shares the key and hits the cache.
	got, err := app.getPopularGenres(context.Background(), 100, 0, startTime.Add(time.Minute))
	require.NoError(t, err)
	assert.Equal(t, seeded, got, "same 15m bucket should hit the seeded entry")

	// A different bucket / limit / offset is a distinct key. With no DB pool a
	// miss would panic, so we assert the panic to prove the key actually differs
	// (rather than silently returning the seeded slice).
	for name, call := range map[string]func() ([]PopularGenre, error){
		"next bucket": func() ([]PopularGenre, error) {
			return app.getPopularGenres(context.Background(), 100, 0, startTime.Add(genresPopularCacheTTL))
		},
		"diff limit": func() ([]PopularGenre, error) {
			return app.getPopularGenres(context.Background(), 50, 0, startTime)
		},
		"diff offset": func() ([]PopularGenre, error) {
			return app.getPopularGenres(context.Background(), 100, 10, startTime)
		},
	} {
		t.Run(name, func(t *testing.T) {
			assert.Panics(t, func() { _, _ = call() }, "distinct key should miss and reach the nil DB pool")
		})
	}
}

func TestGenresPopular(t *testing.T) {
	app := testAppWithFixtures(t)

	var response struct {
		Data []struct {
			Name  string `json:"name"`
			Count int64  `json:"count"`
		}
	}

	status, _ := testGet(t, app, "/v1/genres/popular?start_time=0", &response)
	require.Equal(t, 200, status)
	require.NotEmpty(t, response.Data)

	var foundElectronic bool
	for i, genre := range response.Data {
		if genre.Name == "Electronic" {
			foundElectronic = true
		}
		if i > 0 {
			assert.GreaterOrEqual(t, response.Data[i-1].Count, genre.Count)
		}
	}
	assert.True(t, foundElectronic, "expected fixture genres in response")
}

func TestGenresPopularMinCount(t *testing.T) {
	app := testAppWithFixtures(t)

	var response struct {
		Data []struct {
			Name  string `json:"name"`
			Count int64  `json:"count"`
		}
	}

	status, _ := testGet(t, app, "/v1/genres/popular?start_time=0&min_count=2", &response)
	require.Equal(t, 200, status)
	require.NotEmpty(t, response.Data)

	for _, genre := range response.Data {
		assert.GreaterOrEqual(t, genre.Count, int64(2))
	}
}

func TestGenresPopularExcludesAccessAuthoritiesTracks(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	url := fmt.Sprintf("/v1/genres/popular?start_time=%d", 0)
	var before struct {
		Data []struct {
			Name  string `json:"name"`
			Count int64  `json:"count"`
		}
	}
	status, _ := testGet(t, app, url, &before)
	require.Equal(t, 200, status)

	var electronicCountBefore int64
	for _, g := range before.Data {
		if g.Name == "Electronic" {
			electronicCountBefore = g.Count
			break
		}
	}
	require.Greater(t, electronicCountBefore, int64(0), "fixtures should have Electronic tracks")

	_, err := app.writePool.Exec(ctx, `UPDATE tracks SET access_authorities = ARRAY['0xgate']::text[] WHERE track_id = 100 AND is_current = true`)
	require.NoError(t, err)

	// The endpoint caches its result for genresPopularCacheTTL; clear it so the
	// re-read reflects the DB mutation we just made rather than the cached count.
	app.genresPopularCache.Clear()

	var after struct {
		Data []struct {
			Name  string `json:"name"`
			Count int64  `json:"count"`
		}
	}
	status, _ = testGet(t, app, url, &after)
	require.Equal(t, 200, status)

	var electronicCountAfter int64
	for _, g := range after.Data {
		if g.Name == "Electronic" {
			electronicCountAfter = g.Count
			break
		}
	}
	assert.Equal(t, electronicCountBefore-1, electronicCountAfter, "genre count must exclude access_authorities tracks")
}
