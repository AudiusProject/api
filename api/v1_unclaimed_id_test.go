package api

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

// An edge cache in front of these routes hands one response to every caller,
// so each must tell caches not to store it.
func TestUnclaimedIdAndBlockConfirmationAreNotCacheable(t *testing.T) {
	app := emptyTestApp(t)

	for _, path := range []string{
		"/v1/users/unclaimed_id",
		"/v1/tracks/unclaimed_id",
		"/v1/playlists/unclaimed_id",
		"/v1/playlists/unclaimed-id",
		"/v1/comments/unclaimed_id",
		"/v1/comments/unclaimed-id",
		"/v1/events/unclaimed_id",
		"/v1/events/unclaimed-id",
		"/block_confirmation?blocknumber=1&blockhash=0xabc",
		"/block-confirmation?blocknumber=1&blockhash=0xabc",
	} {
		t.Run(path, func(t *testing.T) {
			res, err := app.Test(httptest.NewRequest("GET", path, nil), -1)
			assert.NoError(t, err)
			assert.Equal(t, 200, res.StatusCode)
			assert.Equal(t, "no-store", res.Header.Get("Cache-Control"))
		})
	}
}
