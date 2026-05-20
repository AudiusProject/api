package jobs

import (
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBasicAuthNonce_Shape verifies the Authorization header has the
// expected "Basic base64(<ts>:<sig_hex>)" structure and decodes cleanly.
// Cross-format verification against Python apps is left to integration
// testing — the signing primitives (keccak + eth personal-sign) are
// covered by go-ethereum's own tests.
func TestBasicAuthNonce_Shape(t *testing.T) {
	// Test key from api/'s default dev config.
	key := "13422b9affd75ff80f94f1ea394e6a6097830cb58cda2d3542f37464ecaee7df"
	now := time.UnixMilli(1700000000000)

	auth, err := basicAuthNonce(key, now)
	require.NoError(t, err)

	require.True(t, strings.HasPrefix(auth, "Basic "))
	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
	require.NoError(t, err)

	parts := strings.SplitN(string(decoded), ":", 2)
	require.Len(t, parts, 2)
	assert.Equal(t, "1700000000000", parts[0])
	// signature is 65 bytes = 130 hex chars
	assert.Len(t, parts[1], 130)
}

func TestBasicAuthNonce_BadKey(t *testing.T) {
	_, err := basicAuthNonce("not-hex", time.Now())
	assert.Error(t, err)
}
