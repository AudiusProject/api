package api

import (
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func latestTestApp(t *testing.T) *ApiServer {
	app := testAppWithFixtures(t)
	database.SeedTable(app.pool.Replicas[0], "tracks", []map[string]any{
		{"track_id": 800, "genre": "LatestTestGenreA", "owner_id": 1, "title": "Latest Track Old", "is_unlisted": "f", "created_at": "2025-01-01 00:00:00"},
		{"track_id": 801, "genre": "LatestTestGenreA", "owner_id": 1, "title": "Latest Track Mid", "is_unlisted": "f", "created_at": "2025-02-01 00:00:00"},
		{"track_id": 802, "genre": "LatestTestGenreC", "owner_id": 1, "title": "Latest Track New", "is_unlisted": "f", "created_at": "2025-03-01 00:00:00"},
		{"track_id": 803, "genre": "LatestTestGenreB", "owner_id": 1, "title": "Latest Track Visible", "is_unlisted": "f", "created_at": "2025-02-15 00:00:00"},
		{"track_id": 804, "genre": "LatestTestGenreB", "owner_id": 1, "title": "Latest Track Hidden", "is_unlisted": "t", "created_at": "2025-02-20 00:00:00"},
	})
	return app
}

func TestGetLatest(t *testing.T) {
	app := latestTestApp(t)
	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/tracks/latest?limit=5", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 5, len(resp.Data))
}

func TestGetLatestWithGenre(t *testing.T) {
	app := latestTestApp(t)
	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/tracks/latest?genre=LatestTestGenreA", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 2, len(resp.Data))
	assert.Equal(t, trashid.MustEncodeHashID(801), resp.Data[0].ID)
	assert.Equal(t, trashid.MustEncodeHashID(800), resp.Data[1].ID)
	for _, track := range resp.Data {
		assert.Equal(t, "LatestTestGenreA", track.Genre.String)
	}
}

func TestGetLatestWithLimitOffset(t *testing.T) {
	app := latestTestApp(t)
	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/tracks/latest?genre=LatestTestGenreA&limit=1&offset=0", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, trashid.MustEncodeHashID(801), resp.Data[0].ID)

	status, _ = testGet(t, app, "/v1/tracks/latest?genre=LatestTestGenreA&limit=1&offset=1", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, trashid.MustEncodeHashID(800), resp.Data[0].ID)
}

func TestGetLatestExcludesUnlisted(t *testing.T) {
	app := latestTestApp(t)
	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/tracks/latest?genre=LatestTestGenreB", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, trashid.MustEncodeHashID(803), resp.Data[0].ID)
}
