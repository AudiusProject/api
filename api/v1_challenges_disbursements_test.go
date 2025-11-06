package api

import (
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetChallengeDisbursements(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"challenge_disbursements": {
			{
				"challenge_id": "e",
				"user_id":      1,
				"specifier":    "def",
				"signature":    "sig123",
				"slot":         102,
				"amount":       "100",
				"created_at":   time.Now().Add(-time.Minute * 3),
			},
			{
				"challenge_id": "f",
				"user_id":      3,
				"specifier":    "jkl:3",
				"signature":    "sig789",
				"slot":         101,
				"amount":       "200",
				"created_at":   time.Now().Add(-time.Minute * 2),
			},
			{
				"challenge_id": "f",
				"user_id":      2,
				"specifier":    "jkl:2",
				"signature":    "sig456",
				"slot":         103,
				"amount":       "200",
				"created_at":   time.Now().Add(-time.Minute * 1),
			},
		},
	}

	database.Seed(app.writePool, fixtures)

	// Default sort created_at desc
	{
		status, body := testGet(t, app, "/v1/challenges/disbursements")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.0.challenge_id": "f",
			"data.0.user_id":      trashid.MustEncodeHashID(2),
		})
	}

	// filter by challenge_id
	{
		status, body := testGet(t, app, "/v1/challenges/disbursements?challenge_id=e")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":              1,
			"data.0.challenge_id": "e",
		})
	}

	// filter by challenge_user_id
	{
		status, body := testGet(t, app, "/v1/challenges/disbursements?challenge_user_id="+trashid.MustEncodeHashID(1))
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":              1,
			"data.0.challenge_id": "e",
		})
	}

	// filter by specifier_query
	{
		status, body := testGet(t, app, "/v1/challenges/disbursements?specifier_query=jkl")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.#":              2,
			"data.0.challenge_id": "f",
			"data.0.user_id":      trashid.MustEncodeHashID(2),
			"data.0.specifier":    "jkl:2",
			"data.1.challenge_id": "f",
			"data.1.user_id":      trashid.MustEncodeHashID(3),
			"data.1.specifier":    "jkl:3",
		})
	}

	// sort by created_at asc
	{
		status, body := testGet(t, app, "/v1/challenges/disbursements?sort_method=created_at&sort_direction=asc")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.0.challenge_id": "e",
			"data.0.user_id":      trashid.MustEncodeHashID(1),
		})
	}

	// sort by slot asc
	{
		status, body := testGet(t, app, "/v1/challenges/disbursements?sort_method=slot&sort_direction=asc")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.0.challenge_id": "f",
			"data.0.user_id":      trashid.MustEncodeHashID(3),
		})
	}

	// sort by slot desc
	{
		status, body := testGet(t, app, "/v1/challenges/disbursements?sort_method=slot&sort_direction=desc")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.0.challenge_id": "f",
			"data.0.user_id":      trashid.MustEncodeHashID(2),
		})
	}

	// sort by amount asc
	{
		status, body := testGet(t, app, "/v1/challenges/disbursements?sort_method=amount&sort_direction=asc")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.0.challenge_id": "e",
			"data.0.user_id":      trashid.MustEncodeHashID(1),
			"data.1.challenge_id": "f",
			"data.1.user_id":      trashid.MustEncodeHashID(2),
			"data.2.challenge_id": "f",
			"data.2.user_id":      trashid.MustEncodeHashID(3),
		})
	}

	// sort by amount desc (user_id tie breaker)
	{
		status, body := testGet(t, app, "/v1/challenges/disbursements?sort_method=amount&sort_direction=desc")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.0.challenge_id": "f",
			"data.0.user_id":      trashid.MustEncodeHashID(2),
			"data.1.challenge_id": "f",
			"data.1.user_id":      trashid.MustEncodeHashID(3),
			"data.2.challenge_id": "e",
			"data.2.user_id":      trashid.MustEncodeHashID(1),
		})
	}

	// sort by specifier asc
	{
		status, body := testGet(t, app, "/v1/challenges/disbursements?specifier_query=jkl&sort_method=specifier&sort_direction=asc")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.0.challenge_id": "f",
			"data.0.user_id":      trashid.MustEncodeHashID(2),
			"data.1.challenge_id": "f",
			"data.1.user_id":      trashid.MustEncodeHashID(3),
		})
	}

	// sort by specifier desc
	{
		status, body := testGet(t, app, "/v1/challenges/disbursements?specifier_query=jkl&sort_method=specifier&sort_direction=desc")
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.0.challenge_id": "f",
			"data.0.user_id":      trashid.MustEncodeHashID(3),
			"data.1.challenge_id": "f",
			"data.1.user_id":      trashid.MustEncodeHashID(2),
		})
	}
}
