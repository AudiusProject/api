package api

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"github.com/stretchr/testify/assert"
)

func TestGetTrack(t *testing.T) {
	var trackResponse struct {
		Data dbv1.FullTrack
	}

	status, body := testGet(t, "/v1/full/tracks/eYJyn", &trackResponse)
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]string{
		"data.id":    "eYJyn",
		"data.title": "Culca Canyon",
	})
}

func TestGetTrackFollowDownloadAcess(t *testing.T) {
	var trackResponse struct {
		Data dbv1.FullTrack
	}
	// No access
	_, body1 := testGet(t, "/v1/full/tracks/eYRWn", &trackResponse)
	jsonAssert(t, body1, map[string]string{
		"data.title":           "Follow Gated Download",
		"data.access.stream":   "true",
		"data.access.download": "false",
	})

	// With access
	// _, body2 := testGet(t, "/v1/full/tracks/eYRWn?user_id=ELKzn", &trackResponse)
	// jsonAssert(t, body2, map[string]string{
	// 	"data.title":           "Follow Gated Download",
	// 	"data.access.stream":   "true",
	// 	"data.access.download": "true",
	// })
}

func TestGetTrackTipStreamAccess(t *testing.T) {
	var trackResponse struct {
		Data dbv1.FullTrack
	}
	// No access
	_, body1 := testGet(t, "/v1/full/tracks/L5x7n", &trackResponse)
	jsonAssert(t, body1, map[string]string{
		"data.title":           "Tip Gated Stream",
		"data.access.stream":   "false",
		"data.access.download": "false",
	})

	// With access
	_, body2 := testGet(t, "/v1/full/tracks/L5x7n?user_id=ELKzn", &trackResponse)
	jsonAssert(t, body2, map[string]string{
		"data.title":           "Tip Gated Stream",
		"data.access.stream":   "true",
		"data.access.download": "true",
	})
}

func TestGetTrackUsdcPurchaseStreamAccess(t *testing.T) {
	var trackResponse struct {
		Data dbv1.FullTrack
	}
	// No access
	_, body1 := testGet(t, "/v1/full/tracks/ebdJL", &trackResponse)
	jsonAssert(t, body1, map[string]string{
		"data.title":           "Pay Gated Stream",
		"data.access.stream":   "false",
		"data.access.download": "false",
	})

	// With access
	_, body2 := testGet(t, "/v1/full/tracks/ebdJL?user_id=1D9On", &trackResponse)
	jsonAssert(t, body2, map[string]string{
		"data.title":           "Pay Gated Stream",
		"data.access.stream":   "true",
		"data.access.download": "false",
	})
}
