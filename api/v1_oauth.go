package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

// oauthError returns an RFC 6749 §5.2 error response.
func oauthError(c *fiber.Ctx, status int, errCode, description string) error {
	return c.Status(status).JSON(fiber.Map{
		"error":             errCode,
		"error_description": description,
	})
}

// generateRandomToken generates a cryptographically random URL-safe base64 string of n bytes.
func generateRandomToken(nBytes int) (string, error) {
	b := make([]byte, nBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// --- Request body structs ---

type oauthAuthorizeBody struct {
	Token               string `json:"token"`
	ClientID            string `json:"client_id"`
	RedirectURI         string `json:"redirect_uri"`
	CodeChallenge       string `json:"code_challenge"`
	CodeChallengeMethod string `json:"code_challenge_method"`
	Scope               string `json:"scope"`
}

type oauthTokenBody struct {
	GrantType    string `json:"grant_type"`
	Code         string `json:"code"`
	CodeVerifier string `json:"code_verifier"`
	ClientID     string `json:"client_id"`
	RedirectURI  string `json:"redirect_uri"`
	RefreshToken string `json:"refresh_token"`
}

type oauthRevokeBody struct {
	Token    string `json:"token"`
	ClientID string `json:"client_id"`
}

// --- Cache entry for PKCE tokens ---

type oauthTokenCacheEntry struct {
	UserID   int32
	ClientID string
	Scope    string
}

// --- Handlers ---

// v1OAuthAuthorize handles POST /v1/oauth/authorize
// Called by the audius.co consent screen after the user authenticates.
func (app *ApiServer) v1OAuthAuthorize(c *fiber.Ctx) error {
	var body oauthAuthorizeBody
	if err := c.BodyParser(&body); err != nil {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "Invalid request body")
	}

	// Validate required fields
	if body.Token == "" || body.ClientID == "" || body.RedirectURI == "" || body.CodeChallenge == "" || body.Scope == "" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "Missing required parameters")
	}

	if body.CodeChallengeMethod != "S256" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "code_challenge_method must be S256")
	}

	if body.Scope != "read" && body.Scope != "write" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "scope must be 'read' or 'write'")
	}

	// 1. Validate JWT via existing logic and also check iat
	userId, err := app.validateOAuthJWTTokenToUserId(c.Context(), body.Token)
	if err != nil {
		return oauthError(c, fiber.StatusUnauthorized, "access_denied", "Invalid JWT token")
	}

	// Validate iat (issued-at) — reject if more than 5 min in the past or in the future
	if err := app.validateJWTIat(body.Token); err != nil {
		return oauthError(c, fiber.StatusUnauthorized, "access_denied", err.Error())
	}

	// 2. Validate client_id exists in developer_apps
	clientID := strings.ToLower(body.ClientID)
	var appExists bool
	err = app.pool.QueryRow(c.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM developer_apps
			WHERE LOWER(address) = $1 AND is_current = true AND NOT is_delete
		)
	`, clientID).Scan(&appExists)
	if err != nil || !appExists {
		return oauthError(c, fiber.StatusBadRequest, "invalid_client", "Unknown client_id")
	}

	// 3. Validate redirect_uri
	if !strings.EqualFold(body.RedirectURI, "postmessage") {
		var uriRegistered bool
		err = app.pool.QueryRow(c.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM oauth_redirect_uris
				WHERE LOWER(client_id) = $1 AND redirect_uri = $2
			)
		`, clientID, body.RedirectURI).Scan(&uriRegistered)
		if err != nil || !uriRegistered {
			return oauthError(c, fiber.StatusBadRequest, "invalid_request", "redirect_uri not registered")
		}
	}

	// 4. If scope is write, check for existing approved grant
	if body.Scope == "write" {
		var grantExists bool
		err = app.pool.QueryRow(c.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM grants
				WHERE user_id = $1
					AND LOWER(grantee_address) = $2
					AND is_current = true
					AND is_approved = true
					AND is_revoked = false
			)
		`, int32(userId), clientID).Scan(&grantExists)
		if err != nil || !grantExists {
			return oauthError(c, fiber.StatusForbidden, "access_denied", "No approved grant exists for this app. The user must grant permission first.")
		}
	}

	// 5. Generate random authorization code
	code, err := generateRandomToken(32)
	if err != nil {
		app.logger.Error("Failed to generate auth code", zap.Error(err))
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to generate authorization code")
	}

	// Insert into oauth_authorization_codes
	_, err = app.writePool.Exec(c.Context(), `
		INSERT INTO oauth_authorization_codes (code, client_id, user_id, redirect_uri, code_challenge, code_challenge_method, scope)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, code, clientID, int32(userId), body.RedirectURI, body.CodeChallenge, body.CodeChallengeMethod, body.Scope)
	if err != nil {
		app.logger.Error("Failed to insert auth code", zap.Error(err))
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to create authorization code")
	}

	// 6. Return the code
	return c.JSON(fiber.Map{
		"code": code,
	})
}

// v1OAuthToken handles POST /v1/oauth/token
// Supports grant_type=authorization_code and grant_type=refresh_token.
func (app *ApiServer) v1OAuthToken(c *fiber.Ctx) error {
	var body oauthTokenBody
	if err := c.BodyParser(&body); err != nil {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "Invalid request body")
	}

	switch body.GrantType {
	case "authorization_code":
		return app.oauthTokenAuthorizationCode(c, &body)
	case "refresh_token":
		return app.oauthTokenRefreshToken(c, &body)
	default:
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "grant_type must be 'authorization_code' or 'refresh_token'")
	}
}

func (app *ApiServer) oauthTokenAuthorizationCode(c *fiber.Ctx, body *oauthTokenBody) error {
	if body.Code == "" || body.CodeVerifier == "" || body.ClientID == "" || body.RedirectURI == "" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "Missing required parameters for authorization_code grant")
	}

	clientID := strings.ToLower(body.ClientID)

	// Atomically consume the code
	var storedClientID, storedRedirectURI, storedCodeChallenge, storedCodeChallengeMethod, storedScope string
	var storedUserID int32
	err := app.writePool.QueryRow(c.Context(), `
		UPDATE oauth_authorization_codes
		SET used = true
		WHERE code = $1 AND used = false AND expires_at > NOW()
		RETURNING client_id, user_id, redirect_uri, code_challenge, code_challenge_method, scope
	`, body.Code).Scan(&storedClientID, &storedUserID, &storedRedirectURI, &storedCodeChallenge, &storedCodeChallengeMethod, &storedScope)
	if err != nil {
		if err == pgx.ErrNoRows {
			return oauthError(c, fiber.StatusBadRequest, "invalid_grant", "Authorization code is invalid, expired, or already used")
		}
		app.logger.Error("Failed to consume auth code", zap.Error(err))
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to process authorization code")
	}

	// Verify client_id matches
	if strings.ToLower(storedClientID) != clientID {
		return oauthError(c, fiber.StatusBadRequest, "invalid_grant", "client_id mismatch")
	}

	// Verify redirect_uri matches
	if storedRedirectURI != body.RedirectURI {
		return oauthError(c, fiber.StatusBadRequest, "invalid_grant", "redirect_uri mismatch")
	}

	// Verify PKCE: base64url(SHA256(code_verifier)) must equal the stored code_challenge
	if storedCodeChallengeMethod != "S256" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_grant", "Unsupported code_challenge_method")
	}
	h := sha256.Sum256([]byte(body.CodeVerifier))
	computedChallenge := base64.RawURLEncoding.EncodeToString(h[:])
	if computedChallenge != storedCodeChallenge {
		return oauthError(c, fiber.StatusBadRequest, "invalid_grant", "PKCE verification failed")
	}

	// Generate family_id, access token, refresh token
	familyID, err := generateRandomToken(32)
	if err != nil {
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to generate tokens")
	}
	accessToken, err := generateRandomToken(32)
	if err != nil {
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to generate tokens")
	}
	refreshToken, err := generateRandomToken(32)
	if err != nil {
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to generate tokens")
	}

	accessExpiresAt := time.Now().Add(1 * time.Hour)
	refreshExpiresAt := time.Now().Add(30 * 24 * time.Hour) // 30 days

	// Insert both tokens
	_, err = app.writePool.Exec(c.Context(), `
		INSERT INTO oauth_tokens (token, token_type, client_id, user_id, scope, expires_at, family_id)
		VALUES ($1, 'access', $2, $3, $4, $5, $6)
	`, accessToken, clientID, storedUserID, storedScope, accessExpiresAt, familyID)
	if err != nil {
		app.logger.Error("Failed to insert access token", zap.Error(err))
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to create tokens")
	}

	_, err = app.writePool.Exec(c.Context(), `
		INSERT INTO oauth_tokens (token, token_type, client_id, user_id, scope, expires_at, family_id, refresh_token_id)
		VALUES ($1, 'refresh', $2, $3, $4, $5, $6, $7)
	`, refreshToken, clientID, storedUserID, storedScope, refreshExpiresAt, familyID, refreshToken)
	if err != nil {
		app.logger.Error("Failed to insert refresh token", zap.Error(err))
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to create tokens")
	}

	return c.JSON(fiber.Map{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": refreshToken,
		"scope":         storedScope,
	})
}

func (app *ApiServer) oauthTokenRefreshToken(c *fiber.Ctx, body *oauthTokenBody) error {
	if body.RefreshToken == "" || body.ClientID == "" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "Missing required parameters for refresh_token grant")
	}

	clientID := strings.ToLower(body.ClientID)

	// First, check if the token exists and whether it triggers reuse detection.
	// We need a two-phase approach: check for reuse first, then atomically consume.
	var storedClientID string
	var storedUserID int32
	var storedScope, storedFamilyID string
	var storedIsRevoked bool
	var storedExpiresAt time.Time
	var storedTokenType string
	err := app.writePool.QueryRow(c.Context(), `
		SELECT client_id, user_id, scope, family_id, is_revoked, expires_at, token_type
		FROM oauth_tokens
		WHERE token = $1
	`, body.RefreshToken).Scan(&storedClientID, &storedUserID, &storedScope, &storedFamilyID, &storedIsRevoked, &storedExpiresAt, &storedTokenType)
	if err != nil {
		if err == pgx.ErrNoRows {
			return oauthError(c, fiber.StatusBadRequest, "invalid_grant", "Invalid refresh token")
		}
		app.logger.Error("Failed to look up refresh token", zap.Error(err))
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to process refresh token")
	}

	// Must be a refresh token type
	if storedTokenType != "refresh" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_grant", "Invalid refresh token")
	}

	// Verify client_id matches
	if strings.ToLower(storedClientID) != clientID {
		return oauthError(c, fiber.StatusBadRequest, "invalid_grant", "client_id mismatch")
	}

	// If the token is already revoked — token reuse detected. Revoke all tokens in the family.
	if storedIsRevoked {
		_, _ = app.writePool.Exec(c.Context(), `
			UPDATE oauth_tokens SET is_revoked = true WHERE family_id = $1
		`, storedFamilyID)
		// Invalidate cache entries for this family
		app.invalidateOAuthTokenCacheByFamily(c.Context(), storedFamilyID)
		app.logger.Warn("Refresh token reuse detected, revoking family",
			zap.String("family_id", storedFamilyID),
			zap.Int32("user_id", storedUserID),
		)
		return oauthError(c, fiber.StatusBadRequest, "invalid_grant", "Token reuse detected")
	}

	// If expired
	if time.Now().After(storedExpiresAt) {
		return oauthError(c, fiber.StatusBadRequest, "invalid_grant", "Refresh token expired")
	}

	// Atomically consume the refresh token to prevent race conditions.
	// If two concurrent requests try to use the same token, only one will succeed.
	var consumed bool
	err = app.writePool.QueryRow(c.Context(), `
		UPDATE oauth_tokens SET is_revoked = true
		WHERE token = $1 AND is_revoked = false
		RETURNING true
	`, body.RefreshToken).Scan(&consumed)
	if err != nil {
		if err == pgx.ErrNoRows {
			// Another concurrent request already consumed this token — treat as reuse
			_, _ = app.writePool.Exec(c.Context(), `
				UPDATE oauth_tokens SET is_revoked = true WHERE family_id = $1
			`, storedFamilyID)
			app.invalidateOAuthTokenCacheByFamily(c.Context(), storedFamilyID)
			app.logger.Warn("Refresh token concurrent reuse detected, revoking family",
				zap.String("family_id", storedFamilyID),
				zap.Int32("user_id", storedUserID),
			)
			return oauthError(c, fiber.StatusBadRequest, "invalid_grant", "Token reuse detected")
		}
		app.logger.Error("Failed to revoke old refresh token", zap.Error(err))
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to revoke old refresh token")
	}

	// Generate new tokens with the same family_id
	accessToken, err := generateRandomToken(32)
	if err != nil {
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to generate tokens")
	}
	refreshToken, err := generateRandomToken(32)
	if err != nil {
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to generate tokens")
	}

	accessExpiresAt := time.Now().Add(1 * time.Hour)
	refreshExpiresAt := time.Now().Add(30 * 24 * time.Hour)

	_, err = app.writePool.Exec(c.Context(), `
		INSERT INTO oauth_tokens (token, token_type, client_id, user_id, scope, expires_at, family_id)
		VALUES ($1, 'access', $2, $3, $4, $5, $6)
	`, accessToken, clientID, storedUserID, storedScope, accessExpiresAt, storedFamilyID)
	if err != nil {
		app.logger.Error("Failed to insert new access token", zap.Error(err))
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to create tokens")
	}

	_, err = app.writePool.Exec(c.Context(), `
		INSERT INTO oauth_tokens (token, token_type, client_id, user_id, scope, expires_at, family_id, refresh_token_id)
		VALUES ($1, 'refresh', $2, $3, $4, $5, $6, $7)
	`, refreshToken, clientID, storedUserID, storedScope, refreshExpiresAt, storedFamilyID, refreshToken)
	if err != nil {
		app.logger.Error("Failed to insert new refresh token", zap.Error(err))
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to create tokens")
	}

	return c.JSON(fiber.Map{
		"access_token":  accessToken,
		"token_type":    "Bearer",
		"expires_in":    3600,
		"refresh_token": refreshToken,
		"scope":         storedScope,
	})
}

// v1OAuthRevoke handles POST /v1/oauth/revoke
// Per RFC 7009 §2.2, always returns 200 regardless of whether the token was valid.
func (app *ApiServer) v1OAuthRevoke(c *fiber.Ctx) error {
	var body oauthRevokeBody
	if err := c.BodyParser(&body); err != nil {
		// Per RFC 7009, even for bad requests we should be lenient,
		// but missing token is an actual error
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "Invalid request body")
	}

	if body.Token == "" {
		return oauthError(c, fiber.StatusBadRequest, "invalid_request", "token is required")
	}

	// Look up the token to get family_id for family-wide revocation
	var familyID string
	err := app.writePool.QueryRow(c.Context(), `
		SELECT family_id FROM oauth_tokens WHERE token = $1
	`, body.Token).Scan(&familyID)
	if err != nil {
		// Token not found or error — per RFC 7009, return 200 anyway
		return c.JSON(fiber.Map{})
	}

	// Revoke all tokens in the family
	_, err = app.writePool.Exec(c.Context(), `
		UPDATE oauth_tokens SET is_revoked = true WHERE family_id = $1
	`, familyID)
	if err != nil {
		app.logger.Error("Failed to revoke token family", zap.Error(err))
	}

	// Invalidate cache
	app.invalidateOAuthTokenCacheByFamily(c.Context(), familyID)

	return c.JSON(fiber.Map{})
}

// v1OAuthMe handles GET /v1/oauth/me
// Returns the authenticated user's profile based on Bearer access token.
func (app *ApiServer) v1OAuthMe(c *fiber.Ctx) error {
	// Extract Bearer token
	authHeader := c.Get("Authorization")
	if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
		return oauthError(c, fiber.StatusUnauthorized, "invalid_token", "Missing or invalid Authorization header")
	}
	token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
	if token == "" {
		return oauthError(c, fiber.StatusUnauthorized, "invalid_token", "Bearer token is empty")
	}

	// Look up the access token (try cache first)
	entry, ok := app.lookupOAuthAccessToken(c, token)
	if !ok {
		return oauthError(c, fiber.StatusUnauthorized, "invalid_token", "Invalid or expired access token")
	}

	// Query user info
	encodedUserId, _ := trashid.EncodeHashId(int(entry.UserID))

	var handle, name string
	var verified bool
	var profilePicture *string
	err := app.pool.QueryRow(c.Context(), `
		SELECT handle, name, is_verified,
			CASE WHEN profile_picture_sizes IS NOT NULL
				THEN CONCAT($2::text, '/content/', profile_picture_sizes, '/150x150.jpg')
				ELSE NULL
			END as profile_picture
		FROM users
		WHERE user_id = $1 AND is_current = true
	`, entry.UserID, app.audiusAppUrl).Scan(&handle, &name, &verified, &profilePicture)
	if err != nil {
		if err == pgx.ErrNoRows {
			return oauthError(c, fiber.StatusNotFound, "invalid_token", "User not found")
		}
		app.logger.Error("Failed to query user for /oauth/me", zap.Error(err))
		return oauthError(c, fiber.StatusInternalServerError, "server_error", "Failed to get user info")
	}

	response := fiber.Map{
		"userId":   encodedUserId,
		"name":     name,
		"handle":   handle,
		"verified": verified,
		"sub":      encodedUserId,
		"iat":      time.Now().Unix(),
	}
	if profilePicture != nil {
		response["profilePicture"] = *profilePicture
	}

	// Try to get email if available
	var email *string
	_ = app.pool.QueryRow(c.Context(), `
		SELECT email_address FROM user_emails WHERE user_id = $1
	`, entry.UserID).Scan(&email)
	if email != nil {
		response["email"] = *email
	}

	return c.JSON(response)
}

// --- Helper methods ---

// validateJWTIat extracts and validates the iat (issued-at) claim from a JWT.
// Returns an error if iat is missing or more than 5 minutes from now.
func (app *ApiServer) validateJWTIat(token string) error {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return fmt.Errorf("invalid JWT format")
	}

	paddedPayload := parts[1]
	if len(paddedPayload)%4 != 0 {
		paddedPayload += strings.Repeat("=", 4-len(paddedPayload)%4)
	}
	payloadBytes, err := base64.URLEncoding.DecodeString(paddedPayload)
	if err != nil {
		return fmt.Errorf("JWT payload could not be decoded")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		return fmt.Errorf("JWT payload could not be unmarshalled")
	}

	iatRaw, exists := payload["iat"]
	if !exists {
		return fmt.Errorf("JWT missing iat claim")
	}

	var iat float64
	switch v := iatRaw.(type) {
	case float64:
		iat = v
	case string:
		// iat might be a string-encoded number
		var parsed float64
		if _, err := fmt.Sscanf(v, "%f", &parsed); err != nil {
			return fmt.Errorf("JWT iat is not a valid number")
		}
		iat = parsed
	default:
		return fmt.Errorf("JWT iat has unexpected type")
	}

	iatTime := time.Unix(int64(iat), 0)
	diff := time.Since(iatTime)
	if diff < 0 {
		diff = -diff
	}
	if diff > 5*time.Minute {
		return fmt.Errorf("JWT iat is too far from current time")
	}

	return nil
}

// lookupOAuthAccessToken checks the cache first, then the database, for a valid access token.
// Returns the cache entry and true if found and valid, false otherwise.
func (app *ApiServer) lookupOAuthAccessToken(c *fiber.Ctx, token string) (oauthTokenCacheEntry, bool) {
	// Check cache
	if app.oauthTokenCache != nil {
		if entry, ok := app.oauthTokenCache.Get(token); ok {
			return entry, true
		}
	}

	// Query database
	var userID int32
	var clientID, scope string
	var expiresAt time.Time
	err := app.pool.QueryRow(c.Context(), `
		SELECT user_id, client_id, scope, expires_at
		FROM oauth_tokens
		WHERE token = $1 AND token_type = 'access' AND is_revoked = false AND expires_at > NOW()
	`, token).Scan(&userID, &clientID, &scope, &expiresAt)
	if err != nil {
		return oauthTokenCacheEntry{}, false
	}

	entry := oauthTokenCacheEntry{
		UserID:   userID,
		ClientID: clientID,
		Scope:    scope,
	}

	// Cache the result
	if app.oauthTokenCache != nil {
		app.oauthTokenCache.Set(token, entry)
	}

	return entry, true
}

// invalidateOAuthTokenCacheByFamily removes all cached tokens belonging to a family.
// Since otter doesn't support iteration/invalidation by value, we query the DB for
// all tokens in the family and delete them from cache individually.
func (app *ApiServer) invalidateOAuthTokenCacheByFamily(ctx context.Context, familyID string) {
	if app.oauthTokenCache == nil {
		return
	}

	rows, err := app.writePool.Query(ctx, `
		SELECT token FROM oauth_tokens WHERE family_id = $1
	`, familyID)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var token string
		if err := rows.Scan(&token); err == nil {
			app.oauthTokenCache.Delete(token)
		}
	}
}
