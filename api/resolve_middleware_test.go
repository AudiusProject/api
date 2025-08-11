package api

import (
	"fmt"
	"testing"

	"bridgerton.audius.co/database"
	"github.com/stretchr/testify/assert"
)

func TestResolveUserHandleToId(t *testing.T) {
	app := emptyTestApp(t)

	fixtures := database.FixtureMap{
		"users": make([]map[string]any, 100),
	}

	for i := range 100 {
		handle := fmt.Sprintf("testuser%d", i+1)
		fixtures["users"][i] = map[string]any{
			"user_id":   i + 1,
			"handle":    handle,
			"handle_lc": handle,
		}
	}

	database.Seed(app.pool.Replicas[0], fixtures)

	// This test verifies we aren't hitting a cache pollution bug due
	// to fiber params not being copied before being used in the cache key. We are
	// intentionally using a large number because the sync.Pool used by Fiber is
	// not deterministic in _when_ it reuses a decoder and causes the bug. In empirical
	// testing on a M1 Max, this happens around ~200 requests.
	for i := range 1000 {
		userId := (i % 100) + 1
		status, body := testGet(t, app, fmt.Sprintf("/v1/full/users/handle/testuser%d", userId))
		assert.Equal(t, 200, status)

		if ok := jsonAssert(t, body, map[string]any{
			"data.0.handle": fmt.Sprintf("testuser%d", userId),
		}); !ok {
			// Bail early on failure so we don't barf out a bunch of errors.
			fmt.Printf("failed on iteration %d\n", i)
			return
		}
	}
}
