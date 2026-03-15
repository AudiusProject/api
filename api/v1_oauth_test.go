package api

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api.audius.co/database"
	"api.audius.co/trashid"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// oauthTestPrivKeyHex is the private key used to sign test JWTs for /v1/oauth/authorize.
// The corresponding Ethereum wallet address is 0x58802e7a660990622b728bf06ae9361bc9403eb7.
const oauthTestPrivKeyHex = "0633fddb74e32b3cbc64382e405146319c11a1a52dc96598e557c5dbe2f31468"

// seedOAuthTestData seeds the database with test data for OAuth tests.
// Uses database.Seed for standard tables and database.SeedTable for OAuth tables.
// Returns the client_id (developer app address).
func seedOAuthTestData(t *testing.T, app *ApiServer) string {
	t.Helper()
	clientID := "0xaabb000000000000000000000000000000000001"

	database.Seed(app.pool.Replicas[0], database.FixtureMap{
		"users": {
			{
				"user_id":   100,
				"handle":    "oauthuser",
				"handle_lc": "oauthuser",
				"wallet":    "0x58802e7a660990622b728bf06ae9361bc9403eb7",
				"name":      "OAuth User",
			},
		},
		"developer_apps": {
			{
				"address":   clientID,
				"user_id":   100,
				"name":      "Test OAuth App",
				"is_delete": false,
			},
		},
		"grants": {
			{
				"user_id":         100,
				"grantee_address": clientID,
				"is_approved":     true,
			},
		},
		"oauth_redirect_uris": {
			{
				"client_id":    clientID,
				"redirect_uri": "https://example.com/callback",
			},
		},
	})

	return clientID
}

// insertTestAuthCode inserts a test auth code and returns the code, code_verifier, and code_challenge.
func insertTestAuthCode(t *testing.T, app *ApiServer, clientID string, userID int, scope string) (code, codeVerifier, codeChallenge string) {
	t.Helper()
	codeVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	h := sha256.Sum256([]byte(codeVerifier))
	codeChallenge = base64.RawURLEncoding.EncodeToString(h[:])

	code = "test-auth-code-" + fmt.Sprintf("%d", time.Now().UnixNano())

	database.SeedTable(app.pool.Replicas[0], "oauth_authorization_codes", []map[string]any{
		{
			"code":           code,
			"client_id":      clientID,
			"user_id":        userID,
			"redirect_uri":   "https://example.com/callback",
			"code_challenge": codeChallenge,
			"scope":          scope,
		},
	})
	return
}

// insertTestTokens inserts an access and refresh token pair.
func insertTestTokens(t *testing.T, app *ApiServer, clientID string, userID int, scope, familyID string, accessExpiresIn, refreshExpiresIn time.Duration) (accessToken, refreshToken string) {
	t.Helper()
	accessToken = "test-access-" + fmt.Sprintf("%d", time.Now().UnixNano())
	refreshToken = "test-refresh-" + fmt.Sprintf("%d", time.Now().UnixNano())

	database.SeedTable(app.pool.Replicas[0], "oauth_tokens", []map[string]any{
		{
			"token":      accessToken,
			"token_type": "access",
			"client_id":  clientID,
			"user_id":    userID,
			"scope":      scope,
			"expires_at": time.Now().Add(accessExpiresIn),
			"family_id":  familyID,
		},
		{
			"token":            refreshToken,
			"token_type":       "refresh",
			"client_id":        clientID,
			"user_id":          userID,
			"scope":            scope,
			"expires_at":       time.Now().Add(refreshExpiresIn),
			"family_id":        familyID,
			"refresh_token_id": refreshToken,
		},
	})

	return
}

// oauthPostJSON is a helper for posting JSON to OAuth endpoints via the app router.
func oauthPostJSON(t *testing.T, app *ApiServer, path string, body map[string]string) (int, []byte) {
	t.Helper()
	jsonBytes, err := json.Marshal(body)
	require.NoError(t, err)
	return testPost(t, app, path, jsonBytes, map[string]string{"Content-Type": "application/json"})
}

// oauthGetWithBearer is a helper for GET requests with a Bearer token.
func oauthGetWithBearer(t *testing.T, app *ApiServer, path, token string) (int, []byte) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := app.Test(req, -1)
	require.NoError(t, err)
	body, _ := io.ReadAll(res.Body)
	return res.StatusCode, body
}

// makeOAuthJWT generates a valid Ethereum-signed JWT for use in /v1/oauth/authorize tests.
// privKeyHex is an unpadded hex-encoded secp256k1 private key.
// The JWT payload contains the base64url-encoded userId and the current unix timestamp as iat.
func makeOAuthJWT(t *testing.T, userID int, privKeyHex string) string {
	t.Helper()
	privKey, err := crypto.HexToECDSA(privKeyHex)
	require.NoError(t, err)

	encodedUID := trashid.MustEncodeHashID(userID)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256"}`))
	payloadJSON := fmt.Sprintf(`{"userId":"%s","iat":%d}`, encodedUID, time.Now().Unix())
	payload := base64.RawURLEncoding.EncodeToString([]byte(payloadJSON))

	message := header + "." + payload
	prefixedMsg := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message))
	finalHash := crypto.Keccak256Hash(prefixedMsg)

	sigBytes, err := crypto.Sign(finalHash.Bytes(), privKey)
	require.NoError(t, err)

	sigHex := "0x" + hex.EncodeToString(sigBytes)
	sigB64 := base64.RawURLEncoding.EncodeToString([]byte(sigHex))

	return header + "." + payload + "." + sigB64
}

// --- /oauth/authorize ---

func TestOAuthAuthorize(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	token := makeOAuthJWT(t, 100, oauthTestPrivKeyHex)
	h := sha256.Sum256([]byte("verifier"))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h[:])

	status, body := oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"token":                 token,
		"client_id":             clientID,
		"redirect_uri":          "https://example.com/callback",
		"code_challenge":        codeChallenge,
		"code_challenge_method": "S256",
		"scope":                 "read",
	})

	assert.Equal(t, 200, status)
	assert.True(t, gjson.GetBytes(body, "code").Exists())
	assert.NotEmpty(t, gjson.GetBytes(body, "code").String())
}

func TestOAuthAuthorize_WriteScope(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	token := makeOAuthJWT(t, 100, oauthTestPrivKeyHex)
	h := sha256.Sum256([]byte("verifier"))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h[:])

	status, body := oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"token":                 token,
		"client_id":             clientID,
		"redirect_uri":          "https://example.com/callback",
		"code_challenge":        codeChallenge,
		"code_challenge_method": "S256",
		"scope":                 "write",
	})

	// User 100 has an approved grant for clientID, so write scope succeeds
	assert.Equal(t, 200, status)
	assert.True(t, gjson.GetBytes(body, "code").Exists())
	assert.NotEmpty(t, gjson.GetBytes(body, "code").String())
}

func TestOAuthAuthorize_MissingToken(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	status, body := oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"client_id":             clientID,
		"redirect_uri":          "https://example.com/callback",
		"code_challenge":        "somechallenge",
		"code_challenge_method": "S256",
		"scope":                 "read",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

func TestOAuthAuthorize_MissingClientID(t *testing.T) {
	app := emptyTestApp(t)
	seedOAuthTestData(t, app)

	token := makeOAuthJWT(t, 100, oauthTestPrivKeyHex)

	status, body := oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"token":                 token,
		"redirect_uri":          "https://example.com/callback",
		"code_challenge":        "somechallenge",
		"code_challenge_method": "S256",
		"scope":                 "read",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

func TestOAuthAuthorize_MissingRedirectURI(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	token := makeOAuthJWT(t, 100, oauthTestPrivKeyHex)

	status, body := oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"token":                 token,
		"client_id":             clientID,
		"code_challenge":        "somechallenge",
		"code_challenge_method": "S256",
		"scope":                 "read",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

func TestOAuthAuthorize_MissingCodeChallenge(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	token := makeOAuthJWT(t, 100, oauthTestPrivKeyHex)

	status, body := oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"token":                 token,
		"client_id":             clientID,
		"redirect_uri":          "https://example.com/callback",
		"code_challenge_method": "S256",
		"scope":                 "read",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

func TestOAuthAuthorize_MissingScope(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	token := makeOAuthJWT(t, 100, oauthTestPrivKeyHex)

	status, body := oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"token":                 token,
		"client_id":             clientID,
		"redirect_uri":          "https://example.com/callback",
		"code_challenge":        "somechallenge",
		"code_challenge_method": "S256",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

func TestOAuthAuthorize_InvalidCodeChallengeMethod(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	// Required fields all present, but code_challenge_method is not S256
	status, body := oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"token":                 "dummy.token.value",
		"client_id":             clientID,
		"redirect_uri":          "https://example.com/callback",
		"code_challenge":        "somechallenge",
		"code_challenge_method": "plain",
		"scope":                 "read",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
	assert.Contains(t, gjson.GetBytes(body, "error_description").String(), "S256")
}

func TestOAuthAuthorize_InvalidScope(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	// Required fields all present, but scope is not read/write
	status, body := oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"token":                 "dummy.token.value",
		"client_id":             clientID,
		"redirect_uri":          "https://example.com/callback",
		"code_challenge":        "somechallenge",
		"code_challenge_method": "S256",
		"scope":                 "admin",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

func TestOAuthAuthorize_InvalidToken(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	// All required fields valid, but token is not a proper signed JWT
	status, body := oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"token":                 "not.a.validjwt",
		"client_id":             clientID,
		"redirect_uri":          "https://example.com/callback",
		"code_challenge":        "somechallenge",
		"code_challenge_method": "S256",
		"scope":                 "read",
	})

	assert.Equal(t, 401, status)
	jsonAssert(t, body, map[string]any{"error": "access_denied"})
}

func TestOAuthAuthorize_UnknownClientID(t *testing.T) {
	app := emptyTestApp(t)
	seedOAuthTestData(t, app)

	token := makeOAuthJWT(t, 100, oauthTestPrivKeyHex)
	h := sha256.Sum256([]byte("verifier"))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h[:])

	status, body := oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"token":                 token,
		"client_id":             "0xdeadbeef000000000000000000000000000000ff",
		"redirect_uri":          "https://example.com/callback",
		"code_challenge":        codeChallenge,
		"code_challenge_method": "S256",
		"scope":                 "read",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_client"})
}

func TestOAuthAuthorize_WriteScope_NoGrant(t *testing.T) {
	app := emptyTestApp(t)
	seedOAuthTestData(t, app)

	// Add a second developer app that has no grant for user 100
	clientIDNoGrant := "0xbbcc000000000000000000000000000000000002"
	database.SeedTable(app.pool.Replicas[0], "developer_apps", []map[string]any{
		{
			"address":   clientIDNoGrant,
			"user_id":   100,
			"name":      "App Without Grant",
			"is_delete": false,
		},
	})

	token := makeOAuthJWT(t, 100, oauthTestPrivKeyHex)
	h := sha256.Sum256([]byte("verifier"))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h[:])

	status, body := oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"token":                 token,
		"client_id":             clientIDNoGrant,
		"redirect_uri":          "https://example.com/callback",
		"code_challenge":        codeChallenge,
		"code_challenge_method": "S256",
		"scope":                 "write",
	})

	assert.Equal(t, 403, status)
	jsonAssert(t, body, map[string]any{"error": "access_denied"})
}

func TestOAuthAuthorize_UnregisteredRedirectURI(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)
	// seedOAuthTestData registers https://example.com/callback for clientID.
	// Submitting a different redirect_uri must be rejected.

	token := makeOAuthJWT(t, 100, oauthTestPrivKeyHex)
	h := sha256.Sum256([]byte("verifier"))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h[:])

	status, body := oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"token":                 token,
		"client_id":             clientID,
		"redirect_uri":          "https://evil.com/callback",
		"code_challenge":        codeChallenge,
		"code_challenge_method": "S256",
		"scope":                 "read",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
	assert.Contains(t, gjson.GetBytes(body, "error_description").String(), "redirect_uri")
}

// seedAppWithNoRedirectURIs registers a developer app with no redirect URIs and returns its client_id.
func seedAppWithNoRedirectURIs(t *testing.T, app *ApiServer) string {
	t.Helper()
	clientID := "0xccdd000000000000000000000000000000000003"
	database.SeedTable(app.pool.Replicas[0], "developer_apps", []map[string]any{
		{
			"address":   clientID,
			"user_id":   100,
			"name":      "App Without Redirect URIs",
			"is_delete": false,
		},
	})
	return clientID
}

func authorizeWithNoURIsApp(t *testing.T, app *ApiServer, redirectURI string) (int, []byte) {
	t.Helper()
	clientID := seedAppWithNoRedirectURIs(t, app)
	token := makeOAuthJWT(t, 100, oauthTestPrivKeyHex)
	h := sha256.Sum256([]byte("verifier"))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h[:])
	return oauthPostJSON(t, app, "/v1/oauth/authorize", map[string]string{
		"token":                 token,
		"client_id":             clientID,
		"redirect_uri":          redirectURI,
		"code_challenge":        codeChallenge,
		"code_challenge_method": "S256",
		"scope":                 "read",
	})
}

func TestOAuthAuthorize_NoRegisteredRedirectURIs_ValidHTTPS(t *testing.T) {
	app := emptyTestApp(t)
	seedOAuthTestData(t, app)
	status, body := authorizeWithNoURIsApp(t, app, "https://any-uri.example.com/callback")
	assert.Equal(t, 200, status)
	assert.NotEmpty(t, gjson.GetBytes(body, "code").String())
}

func TestOAuthAuthorize_NoRegisteredRedirectURIs_ValidHTTP(t *testing.T) {
	app := emptyTestApp(t)
	seedOAuthTestData(t, app)
	status, body := authorizeWithNoURIsApp(t, app, "http://localhost:3000/callback")
	assert.Equal(t, 200, status)
	assert.NotEmpty(t, gjson.GetBytes(body, "code").String())
}

func TestOAuthAuthorize_NoRegisteredRedirectURIs_PostMessage(t *testing.T) {
	app := emptyTestApp(t)
	seedOAuthTestData(t, app)
	status, body := authorizeWithNoURIsApp(t, app, "postMessage")
	assert.Equal(t, 200, status)
	assert.NotEmpty(t, gjson.GetBytes(body, "code").String())
}

func TestOAuthAuthorize_NoRegisteredRedirectURIs_LoopbackIP(t *testing.T) {
	app := emptyTestApp(t)
	seedOAuthTestData(t, app)
	status, body := authorizeWithNoURIsApp(t, app, "http://127.0.0.1:8080/callback")
	assert.Equal(t, 200, status)
	assert.NotEmpty(t, gjson.GetBytes(body, "code").String())
}

func TestOAuthAuthorize_NoRegisteredRedirectURIs_NonHTTPScheme(t *testing.T) {
	app := emptyTestApp(t)
	seedOAuthTestData(t, app)
	status, body := authorizeWithNoURIsApp(t, app, "myapp://oauth/callback")
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

func TestOAuthAuthorize_NoRegisteredRedirectURIs_BareIP(t *testing.T) {
	app := emptyTestApp(t)
	seedOAuthTestData(t, app)
	status, body := authorizeWithNoURIsApp(t, app, "https://192.168.1.1/callback")
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

func TestOAuthAuthorize_NoRegisteredRedirectURIs_Credentials(t *testing.T) {
	app := emptyTestApp(t)
	seedOAuthTestData(t, app)
	status, body := authorizeWithNoURIsApp(t, app, "https://user:pass@evil.com/callback")
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

func TestOAuthAuthorize_NoRegisteredRedirectURIs_Fragment(t *testing.T) {
	app := emptyTestApp(t)
	seedOAuthTestData(t, app)
	status, body := authorizeWithNoURIsApp(t, app, "https://example.com/callback#fragment")
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

func TestOAuthAuthorize_NoRegisteredRedirectURIs_PathTraversal(t *testing.T) {
	app := emptyTestApp(t)
	seedOAuthTestData(t, app)
	status, body := authorizeWithNoURIsApp(t, app, "https://example.com/foo/../../../callback")
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

// --- /oauth/token (authorization_code grant) ---

func TestOAuthTokenAuthorizationCode(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)
	code, codeVerifier, _ := insertTestAuthCode(t, app, clientID, 100, "write")

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"code_verifier": codeVerifier,
		"client_id":     clientID,
		"redirect_uri":  "https://example.com/callback",
	})

	assert.Equal(t, 200, status)
	assert.True(t, gjson.GetBytes(body, "access_token").Exists())
	assert.True(t, gjson.GetBytes(body, "refresh_token").Exists())
	jsonAssert(t, body, map[string]any{
		"token_type": "Bearer",
		"expires_in": float64(3600),
		"scope":      "write",
	})
}

func TestOAuthTokenAuthorizationCode_InvalidCode(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "authorization_code",
		"code":          "nonexistent-code",
		"code_verifier": "whatever",
		"client_id":     clientID,
		"redirect_uri":  "https://example.com/callback",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_grant"})
}

func TestOAuthTokenAuthorizationCode_CodeReuse(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)
	code, codeVerifier, _ := insertTestAuthCode(t, app, clientID, 100, "write")

	reqBody := map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"code_verifier": codeVerifier,
		"client_id":     clientID,
		"redirect_uri":  "https://example.com/callback",
	}

	// First use succeeds
	status1, _ := oauthPostJSON(t, app, "/v1/oauth/token", reqBody)
	assert.Equal(t, 200, status1)

	// Second use fails — code already consumed
	status2, resBody := oauthPostJSON(t, app, "/v1/oauth/token", reqBody)
	assert.Equal(t, 400, status2)
	jsonAssert(t, resBody, map[string]any{"error": "invalid_grant"})
}

func TestOAuthTokenAuthorizationCode_WrongCodeVerifier(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)
	code, _, _ := insertTestAuthCode(t, app, clientID, 100, "write")

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"code_verifier": "wrong-verifier-that-does-not-match",
		"client_id":     clientID,
		"redirect_uri":  "https://example.com/callback",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_grant"})
	assert.Contains(t, gjson.GetBytes(body, "error_description").String(), "PKCE")
}

func TestOAuthTokenAuthorizationCode_ClientIDMismatch(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)
	code, codeVerifier, _ := insertTestAuthCode(t, app, clientID, 100, "write")

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"code_verifier": codeVerifier,
		"client_id":     "0xwrongclient00000000000000000000000000000",
		"redirect_uri":  "https://example.com/callback",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_grant"})
}

func TestOAuthTokenAuthorizationCode_RedirectURIMismatch(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)
	code, codeVerifier, _ := insertTestAuthCode(t, app, clientID, 100, "write")

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"code_verifier": codeVerifier,
		"client_id":     clientID,
		"redirect_uri":  "https://evil.com/callback",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_grant"})
	assert.Contains(t, gjson.GetBytes(body, "error_description").String(), "redirect_uri")
}

func TestOAuthTokenAuthorizationCode_ExpiredCode(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	codeVerifier := "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk"
	h := sha256.Sum256([]byte(codeVerifier))
	codeChallenge := base64.RawURLEncoding.EncodeToString(h[:])
	code := "expired-code-test"

	database.SeedTable(app.pool.Replicas[0], "oauth_authorization_codes", []map[string]any{
		{
			"code":           code,
			"client_id":      clientID,
			"user_id":        100,
			"code_challenge": codeChallenge,
			"scope":          "write",
			"expires_at":     time.Now().Add(-time.Hour),
		},
	})

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"code_verifier": codeVerifier,
		"client_id":     clientID,
		"redirect_uri":  "https://example.com/callback",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_grant"})
}

func TestOAuthTokenAuthorizationCode_MissingParams(t *testing.T) {
	app := emptyTestApp(t)

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type": "authorization_code",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

// --- /oauth/token (refresh_token grant) ---

func TestOAuthTokenRefresh(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	familyID := "test-family-1"
	_, refreshToken := insertTestTokens(t, app, clientID, 100, "write", familyID, time.Hour, 30*24*time.Hour)

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     clientID,
	})

	assert.Equal(t, 200, status)
	assert.True(t, gjson.GetBytes(body, "access_token").Exists())
	assert.True(t, gjson.GetBytes(body, "refresh_token").Exists())
	jsonAssert(t, body, map[string]any{
		"token_type": "Bearer",
		"expires_in": float64(3600),
		"scope":      "write",
	})
	assert.NotEqual(t, refreshToken, gjson.GetBytes(body, "refresh_token").String())
}

func TestOAuthTokenRefresh_ReuseDetection(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	familyID := "test-family-reuse"
	_, refreshToken := insertTestTokens(t, app, clientID, 100, "write", familyID, time.Hour, 30*24*time.Hour)

	reqBody := map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     clientID,
	}

	// First use succeeds
	status1, body1 := oauthPostJSON(t, app, "/v1/oauth/token", reqBody)
	assert.Equal(t, 200, status1)
	newRefreshToken := gjson.GetBytes(body1, "refresh_token").String()

	// Reuse the old refresh token — should trigger reuse detection
	status2, body2 := oauthPostJSON(t, app, "/v1/oauth/token", reqBody)
	assert.Equal(t, 400, status2)
	jsonAssert(t, body2, map[string]any{"error": "invalid_grant"})
	assert.Contains(t, gjson.GetBytes(body2, "error_description").String(), "reuse")

	// The new refresh token from the first rotation should also be revoked (family revocation)
	status3, body3 := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": newRefreshToken,
		"client_id":     clientID,
	})
	assert.Equal(t, 400, status3)
	jsonAssert(t, body3, map[string]any{"error": "invalid_grant"})
}

func TestOAuthTokenRefresh_ExpiredToken(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	familyID := "test-family-expired"
	refreshToken := "expired-refresh-" + fmt.Sprintf("%d", time.Now().UnixNano())

	database.SeedTable(app.pool.Replicas[0], "oauth_tokens", []map[string]any{
		{
			"token":            refreshToken,
			"token_type":       "refresh",
			"client_id":        clientID,
			"user_id":          100,
			"scope":            "write",
			"expires_at":       time.Now().Add(-time.Hour),
			"family_id":        familyID,
			"refresh_token_id": refreshToken,
		},
	})

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     clientID,
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_grant"})
	assert.Contains(t, gjson.GetBytes(body, "error_description").String(), "expired")
}

func TestOAuthTokenRefresh_ClientIDMismatch(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	familyID := "test-family-client-mismatch"
	_, refreshToken := insertTestTokens(t, app, clientID, 100, "write", familyID, time.Hour, 30*24*time.Hour)

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     "0xwrongclient00000000000000000000000000000",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_grant"})
	assert.Contains(t, gjson.GetBytes(body, "error_description").String(), "client_id")
}

func TestOAuthTokenRefresh_AccessTokenNotRefreshable(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	familyID := "test-family-access-not-refreshable"
	accessToken, _ := insertTestTokens(t, app, clientID, 100, "write", familyID, time.Hour, 30*24*time.Hour)

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": accessToken,
		"client_id":     clientID,
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_grant"})
}

func TestOAuthTokenRefresh_InvalidToken(t *testing.T) {
	app := emptyTestApp(t)

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": "nonexistent-token",
		"client_id":     "0xaabb000000000000000000000000000000000001",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_grant"})
}

// --- /oauth/token (invalid grant_type) ---

func TestOAuthToken_InvalidGrantType(t *testing.T) {
	app := emptyTestApp(t)

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type": "client_credentials",
	})

	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

// --- /oauth/revoke ---

func TestOAuthRevoke(t *testing.T) {
	app := emptyTestApp(t)
	ctx := context.Background()
	clientID := seedOAuthTestData(t, app)

	familyID := "test-family-revoke"
	accessToken, refreshToken := insertTestTokens(t, app, clientID, 100, "write", familyID, time.Hour, 30*24*time.Hour)

	status, _ := oauthPostJSON(t, app, "/v1/oauth/revoke", map[string]string{
		"token":     accessToken,
		"client_id": clientID,
	})
	assert.Equal(t, 200, status)

	// Verify both tokens in the family are revoked
	var isRevoked bool
	err := app.writePool.QueryRow(ctx, `
		SELECT is_revoked FROM oauth_tokens WHERE token = $1
	`, accessToken).Scan(&isRevoked)
	require.NoError(t, err)
	assert.True(t, isRevoked, "access token should be revoked")

	err = app.writePool.QueryRow(ctx, `
		SELECT is_revoked FROM oauth_tokens WHERE token = $1
	`, refreshToken).Scan(&isRevoked)
	require.NoError(t, err)
	assert.True(t, isRevoked, "refresh token should be revoked (family revocation)")
}

func TestOAuthRevoke_UnknownToken(t *testing.T) {
	app := emptyTestApp(t)

	// Per RFC 7009, always returns 200 even for unknown tokens
	status, _ := oauthPostJSON(t, app, "/v1/oauth/revoke", map[string]string{
		"token":     "nonexistent-token-xyz",
		"client_id": "0xwhatever",
	})
	assert.Equal(t, 200, status)
}

func TestOAuthRevoke_MissingToken(t *testing.T) {
	app := emptyTestApp(t)

	status, body := oauthPostJSON(t, app, "/v1/oauth/revoke", map[string]string{
		"client_id": "0xwhatever",
	})
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_request"})
}

// --- /oauth/me ---

func TestOAuthMe(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	familyID := "test-family-me"
	accessToken, _ := insertTestTokens(t, app, clientID, 100, "read", familyID, time.Hour, 30*24*time.Hour)

	status, body := oauthGetWithBearer(t, app, "/v1/oauth/me", accessToken)

	assert.Equal(t, 200, status)
	assert.True(t, gjson.GetBytes(body, "userId").Exists())
	jsonAssert(t, body, map[string]any{
		"handle":   "oauthuser",
		"name":     "OAuth User",
		"verified": false,
	})
	assert.Equal(t,
		gjson.GetBytes(body, "userId").String(),
		gjson.GetBytes(body, "sub").String(),
	)
}

func TestOAuthMe_InvalidToken(t *testing.T) {
	app := emptyTestApp(t)

	status, _ := oauthGetWithBearer(t, app, "/v1/oauth/me", "invalid-token")

	// requireAuthMiddleware intercepts before the handler runs
	assert.Equal(t, 401, status)
}

func TestOAuthMe_ExpiredToken(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	expiredToken := "expired-access-" + fmt.Sprintf("%d", time.Now().UnixNano())

	database.SeedTable(app.pool.Replicas[0], "oauth_tokens", []map[string]any{
		{
			"token":      expiredToken,
			"token_type": "access",
			"client_id":  clientID,
			"user_id":    100,
			"scope":      "read",
			"expires_at": time.Now().Add(-time.Hour),
			"family_id":  "expired-family",
		},
	})

	status, _ := oauthGetWithBearer(t, app, "/v1/oauth/me", expiredToken)

	// requireAuthMiddleware intercepts before the handler runs
	assert.Equal(t, 401, status)
}

func TestOAuthMe_RevokedToken(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	revokedToken := "revoked-access-" + fmt.Sprintf("%d", time.Now().UnixNano())

	database.SeedTable(app.pool.Replicas[0], "oauth_tokens", []map[string]any{
		{
			"token":      revokedToken,
			"token_type": "access",
			"client_id":  clientID,
			"user_id":    100,
			"scope":      "read",
			"expires_at": time.Now().Add(time.Hour),
			"family_id":  "revoked-family",
			"is_revoked": true,
		},
	})

	status, _ := oauthGetWithBearer(t, app, "/v1/oauth/me", revokedToken)

	// requireAuthMiddleware intercepts before the handler runs
	assert.Equal(t, 401, status)
}

func TestOAuthMe_MissingAuthHeader(t *testing.T) {
	app := emptyTestApp(t)

	req := httptest.NewRequest("GET", "/v1/oauth/me", nil)
	res, err := app.Test(req, -1)
	require.NoError(t, err)

	// Request goes through requireAuthMiddleware which returns 401 before the handler
	assert.Equal(t, 401, res.StatusCode)
}

// --- Full flow: code exchange → refresh → me → revoke ---

func TestOAuthFullFlow(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	// Step 1: Exchange authorization code for tokens
	code, codeVerifier, _ := insertTestAuthCode(t, app, clientID, 100, "write")

	status, body := oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "authorization_code",
		"code":          code,
		"code_verifier": codeVerifier,
		"client_id":     clientID,
		"redirect_uri":  "https://example.com/callback",
	})
	require.Equal(t, 200, status)
	accessToken := gjson.GetBytes(body, "access_token").String()
	refreshToken := gjson.GetBytes(body, "refresh_token").String()

	// Step 2: Use access token to get user profile
	status, body = oauthGetWithBearer(t, app, "/v1/oauth/me", accessToken)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"handle": "oauthuser"})

	// Step 3: Refresh the token
	status, body = oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": refreshToken,
		"client_id":     clientID,
	})
	require.Equal(t, 200, status)
	newAccessToken := gjson.GetBytes(body, "access_token").String()
	newRefreshToken := gjson.GetBytes(body, "refresh_token").String()
	assert.NotEqual(t, accessToken, newAccessToken)
	assert.NotEqual(t, refreshToken, newRefreshToken)

	// Step 4: New access token works
	status, body = oauthGetWithBearer(t, app, "/v1/oauth/me", newAccessToken)
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"handle": "oauthuser"})

	// Step 5: Revoke
	status, _ = oauthPostJSON(t, app, "/v1/oauth/revoke", map[string]string{
		"token":     newAccessToken,
		"client_id": clientID,
	})
	assert.Equal(t, 200, status)

	// Step 6: Revoked tokens no longer work
	status, _ = oauthGetWithBearer(t, app, "/v1/oauth/me", newAccessToken)
	assert.Equal(t, 401, status)

	// Refreshing with the new refresh token also fails (family revoked)
	status, body = oauthPostJSON(t, app, "/v1/oauth/token", map[string]string{
		"grant_type":    "refresh_token",
		"refresh_token": newRefreshToken,
		"client_id":     clientID,
	})
	assert.Equal(t, 400, status)
	jsonAssert(t, body, map[string]any{"error": "invalid_grant"})
}

// --- validateJWTIat ---

func TestValidateJWTIat(t *testing.T) {
	app := emptyTestApp(t)

	makeJWT := func(iat interface{}) string {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256"}`))
		payload, _ := json.Marshal(map[string]interface{}{"iat": iat, "userId": "abc"})
		payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
		sig := base64.RawURLEncoding.EncodeToString([]byte("fakesig"))
		return header + "." + payloadB64 + "." + sig
	}

	t.Run("valid iat (current time)", func(t *testing.T) {
		token := makeJWT(float64(time.Now().Unix()))
		err := app.validateJWTIat(token)
		assert.NoError(t, err)
	})

	t.Run("iat 4 minutes ago (within window)", func(t *testing.T) {
		token := makeJWT(float64(time.Now().Add(-4 * time.Minute).Unix()))
		err := app.validateJWTIat(token)
		assert.NoError(t, err)
	})

	t.Run("iat 6 minutes ago (outside window)", func(t *testing.T) {
		token := makeJWT(float64(time.Now().Add(-6 * time.Minute).Unix()))
		err := app.validateJWTIat(token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too far")
	})

	t.Run("iat 6 minutes in the future (outside window)", func(t *testing.T) {
		token := makeJWT(float64(time.Now().Add(6 * time.Minute).Unix()))
		err := app.validateJWTIat(token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "too far")
	})

	t.Run("missing iat", func(t *testing.T) {
		header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"ES256"}`))
		payload := base64.RawURLEncoding.EncodeToString([]byte(`{"userId":"abc"}`))
		sig := base64.RawURLEncoding.EncodeToString([]byte("fakesig"))
		token := header + "." + payload + "." + sig
		err := app.validateJWTIat(token)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "missing iat")
	})

	t.Run("iat as string number", func(t *testing.T) {
		token := makeJWT(fmt.Sprintf("%d", time.Now().Unix()))
		err := app.validateJWTIat(token)
		assert.NoError(t, err)
	})

	t.Run("invalid JWT format", func(t *testing.T) {
		err := app.validateJWTIat("not.a.valid-jwt-but-three-parts")
		assert.Error(t, err)
	})
}

// --- generateRandomToken ---

func TestGenerateRandomToken(t *testing.T) {
	token1, err := generateRandomToken(32)
	assert.NoError(t, err)
	assert.NotEmpty(t, token1)
	assert.Equal(t, 43, len(token1)) // 32 bytes -> 43 chars in base64url without padding

	token2, err := generateRandomToken(32)
	assert.NoError(t, err)
	assert.NotEqual(t, token1, token2, "tokens should be unique")
}

// --- lookupOAuthAccessToken + cache ---

func TestLookupOAuthAccessToken_CacheBehavior(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	testApp := fiber.New()
	var lookupResult oauthTokenCacheEntry
	var lookupOK bool

	testApp.Get("/test", func(c *fiber.Ctx) error {
		token := c.Get("X-Token")
		lookupResult, lookupOK = app.lookupOAuthAccessToken(c, token)
		return c.SendStatus(200)
	})

	familyID := "test-cache-family"
	accessToken, _ := insertTestTokens(t, app, clientID, 100, "write", familyID, time.Hour, 30*24*time.Hour)

	// First lookup — cache miss, DB hit
	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Token", accessToken)
	_, err := testApp.Test(req, -1)
	require.NoError(t, err)
	assert.True(t, lookupOK)
	assert.Equal(t, int32(100), lookupResult.UserID)
	assert.Equal(t, strings.ToLower(clientID), strings.ToLower(lookupResult.ClientID))
	assert.Equal(t, "write", lookupResult.Scope)

	// Second lookup — should hit cache
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Token", accessToken)
	_, err = testApp.Test(req, -1)
	require.NoError(t, err)
	assert.True(t, lookupOK)
	assert.Equal(t, int32(100), lookupResult.UserID)

	// Non-existent token should return false
	req = httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("X-Token", "nonexistent-token")
	_, err = testApp.Test(req, -1)
	require.NoError(t, err)
	assert.False(t, lookupOK)
}

// --- PKCE token in auth middleware ---

func TestAuthMiddleware_PKCEToken(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	familyID := "test-family-middleware"
	accessToken, _ := insertTestTokens(t, app, clientID, 100, "write", familyID, time.Hour, 30*24*time.Hour)

	var authedWallet string
	var myId int32

	testApp := fiber.New()
	testApp.Use(app.resolveMyIdMiddleware)
	testApp.Use(app.authMiddleware)
	testApp.Get("/test", func(c *fiber.Ctx) error {
		authedWallet = c.Locals("authedWallet").(string)
		myId = app.getMyId(c)
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res, err := testApp.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, strings.ToLower(clientID), authedWallet)
	assert.Equal(t, int32(100), myId)
}

func TestAuthMiddleware_PKCEToken_InvalidToken(t *testing.T) {
	app := emptyTestApp(t)

	var authedWallet string

	testApp := fiber.New()
	testApp.Use(app.resolveMyIdMiddleware)
	testApp.Use(app.authMiddleware)
	testApp.Get("/test", func(c *fiber.Ctx) error {
		authedWallet = c.Locals("authedWallet").(string)
		return c.SendStatus(200)
	})

	req := httptest.NewRequest("GET", "/test", nil)
	req.Header.Set("Authorization", "Bearer invalid-pkce-token")
	res, err := testApp.Test(req, -1)
	require.NoError(t, err)
	assert.Equal(t, 200, res.StatusCode)
	assert.Equal(t, "", authedWallet)
}

func TestAuthMiddleware_PKCEToken_UserIDMismatch(t *testing.T) {
	app := emptyTestApp(t)
	clientID := seedOAuthTestData(t, app)

	// Token belongs to user 100
	familyID := "test-family-mismatch"
	accessToken, _ := insertTestTokens(t, app, clientID, 100, "write", familyID, time.Hour, 30*24*time.Hour)

	testApp := fiber.New()
	testApp.Use(app.resolveMyIdMiddleware)
	testApp.Use(app.authMiddleware)
	testApp.Get("/test", func(c *fiber.Ctx) error {
		return c.SendStatus(200)
	})

	// Request is for user 200 (different from token's user 100)
	req := httptest.NewRequest("GET", "/test?user_id="+trashid.MustEncodeHashID(200), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	res, err := testApp.Test(req, -1)
	require.NoError(t, err)
	// Should be rejected because the token's user_id (100) does not match the requested user_id (200)
	assert.Equal(t, 403, res.StatusCode)
}
