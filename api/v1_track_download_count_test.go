package api

import (
	"context"
	"fmt"
	"testing"

	"api.audius.co/trashid"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestV1TrackDownloadCount(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	// Track 200 is "Culca Canyon" (eYJyn). Insert two download rows so download_count is 2.
	_, err := app.writePool.Exec(ctx, `
		INSERT INTO track_downloads (txhash, blocknumber, parent_track_id, track_id, user_id)
		VALUES ('tx-dl-1', 101, 200, 200, 1), ('tx-dl-2', 101, 200, 200, 2)
	`)
	require.NoError(t, err)

	status, body := testGet(t, app, "/v1/full/tracks/eYJyn/download_count", nil)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.id":             "eYJyn",
		"data.download_count": 2,
	})
}

func TestV1TracksDownloadCounts(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	// Track 200 (eYJyn) gets 2 downloads; track 201 (eYZmn) has none.
	_, err := app.writePool.Exec(ctx, `
		INSERT INTO track_downloads (txhash, blocknumber, parent_track_id, track_id, user_id)
		VALUES ('tx-dl-1', 101, 200, 200, 1), ('tx-dl-2', 101, 200, 200, 2)
	`)
	require.NoError(t, err)

	status, body := testGet(t, app, "/v1/full/tracks/download_counts?id=eYJyn&id=eYZmn", nil)
	assert.Equal(t, 200, status)

	// Response order is not guaranteed; assert by id.
	data := gjson.GetBytes(body, "data")
	assert.True(t, data.IsArray(), "data should be an array")
	byID := make(map[string]int64)
	for _, el := range data.Array() {
		id := el.Get("id").String()
		byID[id] = el.Get("download_count").Int()
	}
	assert.Equal(t, int64(2), byID["eYJyn"], "eYJyn download_count")
	assert.Equal(t, int64(0), byID["eYZmn"], "eYZmn download_count")
}

func TestV1UserTracksDownloadCount(t *testing.T) {
	app := testAppWithFixtures(t)
	ctx := context.Background()
	require.NotNil(t, app.writePool, "test requires write pool")

	// Track 200 (eYJyn) is owned by user 2. Insert two download rows.
	_, err := app.writePool.Exec(ctx, `
		INSERT INTO track_downloads (txhash, blocknumber, parent_track_id, track_id, user_id)
		VALUES ('tx-user-total-1', 101, 200, 200, 1), ('tx-user-total-2', 101, 200, 200, 2)
	`)
	require.NoError(t, err)

	user2Hash := trashid.MustEncodeHashID(2)
	url := fmt.Sprintf("/v1/full/users/%s/tracks/download_count", user2Hash)
	status, body := testGet(t, app, url, nil)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data": 2,
	})
}
