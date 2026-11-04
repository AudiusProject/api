package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The caches are package-level, so each run needs a cid no earlier run saw.
func uniqueCid(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func seedWaveformTrack(t *testing.T, cid string, hosts ...string) *ApiServer {
	app := emptyTestApp(t)
	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"tracks": []map[string]any{
			{"track_id": 1, "owner_id": 1, "title": "Waveform", "track_cid": cid},
		},
		"users": []map[string]any{
			{"user_id": 1, "handle": "testuser1"},
		},
	})
	app.waveformHostsOverride = func(string) []string { return hosts }
	return app
}

func TestGetTrackWaveform(t *testing.T) {
	cid := uniqueCid("hit")
	var calls atomic.Int32
	miss := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer miss.Close()
	hit := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		assert.Equal(t, "/waveform/"+cid, r.URL.Path)
		assert.Equal(t, "true", r.URL.Query().Get("localOnly"))
		// Peaks is []byte on the node, so it is base64 on the wire.
		w.Write([]byte(`{"buckets":3,"duration_ms":1500,"peaks":"AH//"}`))
	}))
	defer hit.Close()
	app := seedWaveformTrack(t, cid, miss.URL, hit.URL)
	for range 2 {
		req := httptest.NewRequest("GET", "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/waveform", nil)
		res, err := app.Test(req, -1)
		require.NoError(t, err)
		require.Equal(t, 200, res.StatusCode)
		assert.Equal(t, "public, max-age=86400", res.Header.Get("Cache-Control"))

		body, _ := io.ReadAll(res.Body)
		var out struct{ Data waveform }
		require.NoError(t, json.Unmarshal(body, &out))
		assert.Equal(t, []int{0, 127, 255}, out.Data.Peaks)
		assert.Equal(t, 3, out.Data.Buckets)
		assert.Equal(t, int64(1500), out.Data.DurationMs)
	}
	assert.Equal(t, int32(1), calls.Load(), "second request should come from cache")
}

func TestGetTrackWaveform_Miss(t *testing.T) {
	var calls atomic.Int32
	miss := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer miss.Close()
	app := seedWaveformTrack(t, uniqueCid("miss"), miss.URL)
	for range 2 {
		req := httptest.NewRequest("GET", "/v1/tracks/"+trashid.MustEncodeHashID(1)+"/waveform", nil)
		res, err := app.Test(req, -1)
		require.NoError(t, err)
		assert.Equal(t, 404, res.StatusCode)
		assert.Equal(t, "public, max-age=600", res.Header.Get("Cache-Control"))
	}
	assert.Equal(t, int32(1), calls.Load(), "a miss should be cached")
}

func TestGetTrackWaveform_UnknownTrack(t *testing.T) {
	app := seedWaveformTrack(t, "cid-unused")
	req := httptest.NewRequest("GET", "/v1/tracks/"+trashid.MustEncodeHashID(2)+"/waveform", nil)
	res, err := app.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, 404, res.StatusCode)
}
