package api

import (
	"testing"

	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrendingIds(t *testing.T) {
	app := testAppWithFixtures(t)
	var resp struct {
		Data struct {
			Week  []hashIdResponse `json:"week"`
			Month []hashIdResponse `json:"month"`
			Year  []hashIdResponse `json:"year"`
		} `json:"data"`
	}
	status, _ := testGet(t, app, "/v1/tracks/trending/ids", &resp)
	assert.Equal(t, 200, status)

	assert.Equal(t, trashid.MustEncodeHashID(300), resp.Data.Week[0].ID)
	assert.Equal(t, trashid.MustEncodeHashID(202), resp.Data.Week[1].ID)
	assert.Equal(t, trashid.MustEncodeHashID(201), resp.Data.Week[2].ID)
	assert.Equal(t, trashid.MustEncodeHashID(200), resp.Data.Week[3].ID)
	assert.Equal(t, trashid.MustEncodeHashID(400), resp.Data.Month[0].ID)
	assert.Equal(t, trashid.MustEncodeHashID(200), resp.Data.Year[0].ID)
	assert.Equal(t, trashid.MustEncodeHashID(300), resp.Data.Year[1].ID)
}
