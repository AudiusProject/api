package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestV1UsersRelated(t *testing.T) {
	// todo: there's got to be a better way!
	{
		_, err := app.pool.Exec(t.Context(), `update aggregate_user set dominant_genre = 'Electronic'`)
		assert.NoError(t, err)
	}

	var userResponse struct {
		Data []dbv1.FullUser
	}

	{
		status, _ := testGet(t, "/v1/users/7eP5n/related", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 2)
		assert.Equal(t, "stereosteve", userResponse.Data[0].Handle.String)
		assert.Equal(t, "someseller", userResponse.Data[1].Handle.String)
	}

	{
		status, _ := testGet(t, "/v1/users/7eP5n/related?user_id=7eP5n&filter_followed=true", &userResponse)
		assert.Equal(t, 200, status)
		assert.Len(t, userResponse.Data, 1)
		assert.Equal(t, "someseller", userResponse.Data[0].Handle.String)
	}
}
