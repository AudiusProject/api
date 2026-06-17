package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
