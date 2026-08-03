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
	// The signature must be "0x"-prefixed to match apps' nonce format
	// (f"{ts}:{HexBytes.hex()}" with hexbytes 0.3.1, which prepends "0x").
	// The notifier strips this prefix before decoding; without it the sig
	// bytes shift and verification fails with "recovery param is more than
	// two bits". 2 chars ("0x") + 65 bytes * 2 = 132.
	assert.True(t, strings.HasPrefix(parts[1], "0x"),
		"signature hex must carry 0x prefix, got %q", parts[1])
	assert.Len(t, parts[1], 132)

	// The recovery byte (last hex pair) must be 1b or 1c (27/28), matching
	// web3.py's sign_message convention that apps and the notifier expect.
	recoveryByte := parts[1][len(parts[1])-2:]
	assert.Contains(t, []string{"1b", "1c"}, recoveryByte,
		"recovery byte should be 27/28, got %q", recoveryByte)
}

func TestBasicAuthNonce_BadKey(t *testing.T) {
	_, err := basicAuthNonce("not-hex", time.Now())
	assert.Error(t, err)
}
