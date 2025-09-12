package api

import (
	"fmt"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersEmailsKey(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"email_access": []map[string]any{
			{
				"id":                  1,
				"email_owner_user_id": 1,
				"receiving_user_id":   2,
				"grantor_user_id":     1,
				"encrypted_key":       "test",
				"is_initial":          true,
				"created_at":          time.Now(),
				"updated_at":          time.Now(),
			},
		},
	}
	database.Seed(app.writePool, fixtures)

	// happy path
	{
		status, body := testGet(t, app, fmt.Sprintf("/v1/users/%s/emails/%s/key", trashid.MustEncodeHashID(2), trashid.MustEncodeHashID(1)))
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data.id":                  1,
			"data.email_owner_user_id": 1,
			"data.receiving_user_id":   2,
			"data.grantor_user_id":     1,
			"data.encrypted_key":       "test",
			"data.is_initial":          true,
		})
	}

	// Nonexistent values return null
	{
		status, body := testGet(t, app, fmt.Sprintf("/v1/users/%s/emails/%s/key", trashid.MustEncodeHashID(1), trashid.MustEncodeHashID(3)))
		assert.Equal(t, 200, status)
		jsonAssert(t, body, map[string]any{
			"data": nil,
		})
	}
}
