package api

import (
	"testing"

	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/tidwall/gjson"
)

func TestV1EventsUnclaimedID(t *testing.T) {
	app := emptyTestApp(t)

	status, body := testGet(t, app, "/v1/events/unclaimed_id")
	assert.Equal(t, 200, status)

	id := gjson.GetBytes(body, "data").String()
	assert.NotEmpty(t, id)

	decoded, err := trashid.DecodeHashId(id)
	assert.NoError(t, err)
	assert.Greater(t, decoded, 0)
}

func TestV1EventsUnclaimedIDAlias(t *testing.T) {
	app := emptyTestApp(t)

	status, body := testGet(t, app, "/v1/events/unclaimed-id")
	assert.Equal(t, 200, status)

	id := gjson.GetBytes(body, "data").String()
	assert.NotEmpty(t, id)

	decoded, err := trashid.DecodeHashId(id)
	assert.NoError(t, err)
	assert.Greater(t, decoded, 0)
}
