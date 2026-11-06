package api

import (
	"testing"

	"api.audius.co/api/dbv1"
	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func feedbackTestApp(t *testing.T) *ApiServer {
	app := testAppWithFixtures(t)
	database.SeedTable(app.pool.Replicas[0], "users", []map[string]any{
		{"user_id": 8900, "handle": "feedbackdeactivated", "is_deactivated": true},
	})
	database.SeedTable(app.pool.Replicas[0], "tracks", []map[string]any{
		{"track_id": 900, "genre": "FeedbackGenreA", "owner_id": 1, "title": "Asked First", "feedback_requested_at": "2026-01-01 00:00:00"},
		{"track_id": 901, "genre": "FeedbackGenreA", "owner_id": 1, "title": "Asked Last", "feedback_requested_at": "2026-03-01 00:00:00"},
		{"track_id": 902, "genre": "FeedbackGenreB", "owner_id": 1, "title": "Asked Middle", "feedback_requested_at": "2026-02-01 00:00:00"},
		{"track_id": 903, "genre": "FeedbackGenreA", "owner_id": 1, "title": "Not Asking"},
		{"track_id": 904, "genre": "FeedbackGenreA", "owner_id": 1, "title": "Unlisted", "is_unlisted": "t", "feedback_requested_at": "2026-03-02 00:00:00"},
		{"track_id": 905, "genre": "FeedbackGenreA", "owner_id": 1, "title": "Comments Off", "comments_disabled": "t", "feedback_requested_at": "2026-03-03 00:00:00"},
		{"track_id": 906, "genre": "FeedbackGenreA", "owner_id": 8900, "title": "Deactivated Owner", "feedback_requested_at": "2026-03-04 00:00:00"},
	})
	return app
}

func TestGetFeedbackTracks(t *testing.T) {
	app := feedbackTestApp(t)
	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/tracks/feedback", &resp)
	assert.Equal(t, 200, status)

	ids := []string{}
	for _, track := range resp.Data {
		ids = append(ids, track.ID)
		assert.True(t, track.FeedbackRequested)
		assert.NotNil(t, track.FeedbackRequestedAt)
	}
	assert.Equal(t, []string{
		trashid.MustEncodeHashID(901),
		trashid.MustEncodeHashID(902),
		trashid.MustEncodeHashID(900),
	}, ids)
}

func TestGetFeedbackTracksGenreAndPaging(t *testing.T) {
	app := feedbackTestApp(t)
	var resp struct {
		Data []dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/tracks/feedback?genre=FeedbackGenreA&limit=1&offset=1", &resp)
	assert.Equal(t, 200, status)
	assert.Equal(t, 1, len(resp.Data))
	assert.Equal(t, trashid.MustEncodeHashID(900), resp.Data[0].ID)
}

func TestTrackFeedbackRequestedField(t *testing.T) {
	app := feedbackTestApp(t)
	var resp struct {
		Data dbv1.Track
	}
	status, _ := testGet(t, app, "/v1/tracks/"+trashid.MustEncodeHashID(903), &resp)
	assert.Equal(t, 200, status)
	assert.False(t, resp.Data.FeedbackRequested)
	assert.Nil(t, resp.Data.FeedbackRequestedAt)
}
