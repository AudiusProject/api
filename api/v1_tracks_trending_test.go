package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/stretchr/testify/assert"
)

func TestGetTrending(t *testing.T) {
	var resp struct {
		Data []dbv1.FullTrack
	}
	status, _ := testGet(t, "/v1/tracks/trending", &resp)
	assert.Equal(t, 200, status)

	assert.Equal(t, trashid.HashId(300), resp.Data[0].ID)
	assert.Equal(t, "Electronic", resp.Data[0].Genre.String)

	assert.Equal(t, trashid.HashId(202), resp.Data[1].ID)
	assert.Equal(t, "Alternative", resp.Data[1].Genre.String)

	assert.Equal(t, trashid.HashId(200), resp.Data[2].ID)
	assert.Equal(t, "Electronic", resp.Data[2].Genre.String)
}

func TestGetTrendingElectronic(t *testing.T) {
	status, body := testGet(t, "/v1/tracks/trending?genre=Electronic")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]string{
		"data.0.id":    "eYRWn",
		"data.0.genre": "Electronic",
		"data.1.id":    "eYJyn",
		"data.1.genre": "Electronic",
	})
}

func TestGetTrendingAllTime(t *testing.T) {
	status, body := testGet(t, "/v1/tracks/trending?time=allTime")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]string{
		"data.0.id":    "eYJyn",
		"data.0.genre": "Electronic",
		"data.1.id":    "eYRWn",
		"data.1.genre": "Electronic",
	})
}
