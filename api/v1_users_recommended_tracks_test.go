package api

import (
	"fmt"
	"testing"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersRecommendedTracks(t *testing.T) {
	app := emptyTestApp(t)

	users := make([]map[string]any, 10)
	for i := range 10 {
		users[i] = map[string]any{
			"user_id": i + 1,
			"handle":  fmt.Sprintf("testUser%d", i+1),
		}
	}

	tracks := make([]map[string]any, 10)
	for i := range 10 {
		tracks[i] = map[string]any{
			"track_id": i + 1,
			"owner_id": i + 1,
			"title":    fmt.Sprintf("testTrack%d", i+1),
		}
	}
	tracks[0]["genre"] = "Rock"
	tracks[1]["genre"] = "Pop"
	tracks[2]["genre"] = "Hip Hop"
	tracks[6]["genre"] = "Rock"
	tracks[7]["genre"] = "Pop"
	tracks[8]["genre"] = "Hip Hop"

	trackTrendingScores := make([]map[string]any, 10)
	for i := range 10 {
		trackTrendingScores[i] = map[string]any{
			"track_id":   i + 1,
			"genre":      tracks[i]["genre"],
			"score":      10000000000 - i*100,
			"time_range": "week",
		}
	}

	plays := make([]map[string]any, 3)
	for i := range 3 {
		plays[i] = map[string]any{
			"id":           i,
			"user_id":      1,
			"play_item_id": i + 1,
			"created_at":   time.Now(),
		}
	}
	createFixtures(app, FixtureMap{
		"users":                 users,
		"tracks":                tracks,
		"track_trending_scores": trackTrendingScores,
		"plays":                 plays,
	})

	var response struct {
		Data []dbv1.FullTrack
	}

	status, body := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(1)+"/recommended-tracks", &response)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#": 3,
	})

	// Verify that the response only contains tracks with track_ids 7, 8, and 9
	// Rest are excluded due to being played or not being in the top genres
	expectedTrackIDs := map[int32]bool{7: true, 8: true, 9: true}

	for _, track := range response.Data {
		assert.True(t, expectedTrackIDs[track.TrackID],
			fmt.Sprintf("Unexpected track_id %d found in response", track.TrackID))
	}
}

func TestV1UsersRecommendedTracksInvalidParams(t *testing.T) {
	app := emptyTestApp(t)

	for _, val := range []string{"-1", "101", "invalid"} {
		status, _ := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(1)+"/recommended-tracks?limit="+val)
		assert.Equal(t, 400, status)
	}

	for _, val := range []string{"-1", "invalid"} {
		status, _ := testGet(t, app, "/v1/full/users/"+trashid.MustEncodeHashID(1)+"/recommended-tracks?offset="+val)
		assert.Equal(t, 400, status)
	}
}
