package api

import (
	"testing"

	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetUserFavorites(t *testing.T) {
	app := testAppWithFixtures(t)
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/favorites")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#":                  "1",
		"data.0.favorite_item_id": "100",
		"data.0.favorite_type":    "SaveType.track",
		"data.0.user_id":          "1",
	})
}

func TestGetUserFavoritesEmpty(t *testing.T) {
	app := testAppWithFixtures(t)
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(4)+"/favorites")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data": "[]",
	})
}

func TestGetUserFavoritesQueryParams(t *testing.T) {
	app := testAppWithFixtures(t)
	status, body := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/favorites?limit=1&offset=0")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#":                  "1",
		"data.0.favorite_item_id": "100",
		"data.0.favorite_type":    "SaveType.track",
		"data.0.user_id":          "1",
	})
}

func TestGetUserFavoritesInvalidQueryParams(t *testing.T) {
	app := testAppWithFixtures(t)
	status, _ := testGet(t, app, "/v1/users/"+trashid.MustEncodeHashID(1)+"/favorites?limit=invalid&offset=-1")
	assert.Equal(t, 400, status)
}
