package api

import (
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrendingUndergroundWinnersTracks(t *testing.T) {
	app := testAppWithFixtures(t)

	var resp struct {
		Data []dbv1.Track
	}

	// Test with explicit week - uses trending_results for 2022-01-21
	status, body := testGet(t, app, "/v1/tracks/trending/underground/winners?week=2022-01-21", &resp)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#": 3,
	})

	// Verify order: 519 (rank 1), 520 (rank 2), 521 (rank 3)
	assert.Equal(t, trashid.MustEncodeHashID(519), resp.Data[0].ID)
	assert.Equal(t, "Jazz", resp.Data[0].Genre.String)

	assert.Equal(t, trashid.MustEncodeHashID(520), resp.Data[1].ID)
	assert.Equal(t, "Classical", resp.Data[1].Genre.String)

	assert.Equal(t, trashid.MustEncodeHashID(521), resp.Data[2].ID)
	assert.Equal(t, "Electronic", resp.Data[2].Genre.String)
}

func TestGetTrendingUndergroundWinnersTracksDefaultWeek(t *testing.T) {
	app := testAppWithFixtures(t)

	var resp struct {
		Data []dbv1.Track
	}

	// Without week param - defaults to most recent week in table before today (2022-01-21)
	status, body := testGet(t, app, "/v1/tracks/trending/underground/winners", &resp)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#": 3,
	})

	assert.Equal(t, trashid.MustEncodeHashID(519), resp.Data[0].ID)
	assert.Equal(t, trashid.MustEncodeHashID(520), resp.Data[1].ID)
	assert.Equal(t, trashid.MustEncodeHashID(521), resp.Data[2].ID)
}

func TestGetTrendingUndergroundWinnersTracksOldDateUsesEarliestWeek(t *testing.T) {
	app := testAppWithFixtures(t)

	var resp struct {
		Data []dbv1.Track
	}

	// Query before any data (2020-01-01): nearest row on or after is 2022-01-21 (earliest we have)
	status, _ := testGet(t, app, "/v1/tracks/trending/underground/winners?week=2020-01-01", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 3, len(resp.Data))
}

func TestGetTrendingUndergroundWinnersTracksNearestWeek(t *testing.T) {
	app := testAppWithFixtures(t)

	var resp struct {
		Data []dbv1.Track
	}

	// Query 2022-01-20: nearest row on or after is 2022-01-21
	status, _ := testGet(t, app, "/v1/tracks/trending/underground/winners?week=2022-01-20", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 3, len(resp.Data))
	assert.Equal(t, trashid.MustEncodeHashID(519), resp.Data[0].ID)

	// Query 2022-01-21: exact match
	status, _ = testGet(t, app, "/v1/tracks/trending/underground/winners?week=2022-01-21", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 3, len(resp.Data))

	// Query 2022-01-25: no row after 2022-01-21, use most recent (2022-01-21)
	status, _ = testGet(t, app, "/v1/tracks/trending/underground/winners?week=2022-01-25", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 3, len(resp.Data))
	assert.Equal(t, trashid.MustEncodeHashID(519), resp.Data[0].ID)
}
