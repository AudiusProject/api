package database_test

import (
	"testing"

	"bridgerton.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestSeed(t *testing.T) {
	pool := database.CreateTestDatabase(t)
	fixtures := database.FixtureMap{
		"users": {
			{
				"user_id": 1001,
				"handle":  "test 123",
			},
			{
				"user_id": 1002,
				"handle":  "test 999",
			},
		},
		"tracks": {
			{
				"track_id": 1001,
				"owner_id": 1001,
				"title":    "my jam",
			},
		},
		"follows": {
			{"follower_user_id": 1001, "followee_user_id": 1002},
		},
	}

	database.Seed(pool, fixtures)

	var handle string
	err := pool.QueryRow(t.Context(), `
			SELECT handle FROM users WHERE user_id = 1001
		`).Scan(&handle)
	assert.NoError(t, err)
	assert.Equal(t, "test 123", handle)

	var trackTitle string
	err = pool.QueryRow(t.Context(), `
			SELECT title FROM tracks WHERE track_id = 1001
		`).Scan(&trackTitle)
	assert.NoError(t, err)
	assert.Equal(t, "my jam", trackTitle)

	var follower int32
	err = pool.QueryRow(t.Context(), `
			SELECT follower_user_id FROM follows WHERE followee_user_id = 1002
		`).Scan(&follower)
	assert.NoError(t, err)
	assert.Equal(t, int32(1001), follower)

}
