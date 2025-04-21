package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrending(t *testing.T) {
	app := fixturesTestApp(t)

	var resp struct {
		Data []dbv1.FullTrack
	}
	status, _ := testGet(t, app, "/v1/tracks/trending", &resp)
	assert.Equal(t, 200, status)

	assert.Equal(t, trashid.MustEncodeHashID(300), resp.Data[0].ID)
	assert.Equal(t, "Electronic", resp.Data[0].Genre.String)

	assert.Equal(t, trashid.MustEncodeHashID(202), resp.Data[1].ID)
	assert.Equal(t, "Alternative", resp.Data[1].Genre.String)

	assert.Equal(t, trashid.MustEncodeHashID(200), resp.Data[2].ID)
	assert.Equal(t, "Electronic", resp.Data[2].Genre.String)
}

func TestGetTrendingElectronic(t *testing.T) {
	app := fixturesTestApp(t)

	var resp struct {
		Data []dbv1.FullTrack
	}
	status, _ := testGet(t, app, "/v1/tracks/trending?genre=Electronic", &resp)
	assert.Equal(t, 200, status)

	assert.Equal(t, "eYRWn", resp.Data[0].ID)
	assert.Equal(t, "Electronic", resp.Data[0].Genre.String)

	assert.Equal(t, "eYJyn", resp.Data[1].ID)
	assert.Equal(t, "Electronic", resp.Data[1].Genre.String)
}

func TestGetTrendingAllTime(t *testing.T) {
	app := fixturesTestApp(t)

	var resp struct {
		Data []dbv1.FullTrack
	}
	status, _ := testGet(t, app, "/v1/tracks/trending?time=allTime", &resp)
	assert.Equal(t, 200, status)

	assert.Equal(t, "eYJyn", resp.Data[0].ID)
	assert.Equal(t, "Electronic", resp.Data[0].Genre.String)

	assert.Equal(t, "eYRWn", resp.Data[1].ID)
	assert.Equal(t, "Electronic", resp.Data[1].Genre.String)
}
