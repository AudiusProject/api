package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/database"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1TracksFeelingLucky(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": []map[string]any{
			{
				"user_id": 1,
				"handle":  "user1",
				"name":    "User 1",
			},
			{
				"user_id": 2,
				"handle":  "user2",
				"name":    "User 2",
			},
			{
				"user_id": 3,
				"handle":  "user3",
				"name":    "User 3",
			},
		},
		"follows": []map[string]any{
			{
				"follower_user_id": 1,
				"followee_user_id": 3,
			},
			{
				"follower_user_id": 2,
				"followee_user_id": 3,
			},
		},
		"tracks": []map[string]any{
			{
				"track_id": 1,
				"title":    "Track 1",
				"owner_id": 1,
			},
			{
				"track_id": 2,
				"title":    "Track 2",
				"owner_id": 2,
			},
			{
				"track_id": 3,
				"title":    "Track 3",
				"owner_id": 3,
			},
		},
		"aggregate_plays": []map[string]any{
			{
				"play_item_id": 1,
				"count":        100,
			},
			{
				"play_item_id": 2,
				"count":        250,
			},
			{
				"play_item_id": 3,
				"count":        250,
			},
		},
	}

	database.Seed(app.pool, fixtures)

	{
		var resp struct {
			Data []dbv1.FullTrack
		}

		status, body := testGet(t, app, "/v1/tracks/feeling-lucky", &resp)
		assert.Equal(t, 200, status)

		jsonAssert(t, body, map[string]any{
			"data.#": 2,
		})

		for _, track := range resp.Data {
			assert.NotEqual(t, int32(1), track.TrackID, "Track 1 should not be returned (less than 250 plays)")
		}
	}

	{
		status, body := testGet(t, app, "/v1/tracks/feeling-lucky?min_followers=1")
		assert.Equal(t, 200, status)

		// Should only be track 3 due to follower requirement
		jsonAssert(t, body, map[string]any{
			"data.#":    1,
			"data.0.id": trashid.MustEncodeHashID(3),
		})
	}
}
