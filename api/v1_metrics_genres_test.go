package api

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsGenres(t *testing.T) {
	app := testAppWithFixtures(t)
	// Calculate timestamp for 1 hour ago
	oneHourAgo := time.Now().Add(-1 * time.Hour).Unix()
	url := fmt.Sprintf("/v1/metrics/genres?start_time=%d", oneHourAgo)

	var response struct {
		Data []struct {
			Genre string `json:"genre"`
			Count int    `json:"count"`
		}
	}

	status, _ := testGet(t, app, url, &response)
	assert.Equal(t, 200, status)

	// Find the Electronic genre in the response
	for _, genre := range response.Data {
		if genre.Genre == "Electronic" {
			assert.Greater(t, genre.Count, 0, "Expected at least one Electronic track")
			break
		}
		if genre.Genre == "Alternative" {
			assert.Greater(t, genre.Count, 0, "Expected at least one Alternative track")
			break
		}
		if genre.Genre == "Folk" {
			assert.Greater(t, genre.Count, 0, "Expected at least one Folk track")
			break
		}
	}

}

func TestMetricsGenresExcludesAccessAuthoritiesTracks(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	// Get baseline Electronic count (use epoch so all fixture tracks are included)
	url := fmt.Sprintf("/v1/metrics/genres?start_time=%d", 0)
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

	// Gate one Electronic track (track 100 is Electronic)
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

