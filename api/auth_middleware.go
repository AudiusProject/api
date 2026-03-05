package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	comms "api.audius.co/api/comms"
	"api.audius.co/trashid"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/jackc/pgx/v5"
)

// Recover user id and wallet from signature headers
func (app *ApiServer) recoverAuthorityFromSignatureHeaders(c *fiber.Ctx) string {
	message := c.Get("Encoded-Data-Message")
	signature := c.Get("Encoded-Data-Signature")
	if message == "" || signature == "" {
		// Some callers pass these as query params, check those if not in headers
		message = c.Query("user_data")
		signature = c.Query("user_signature")
		if message == "" || signature == "" {
			return ""
		}
	}

	encodedToRecover := []byte(message)
	prefixedMessage := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(encodedToRecover), encodedToRecover))
	finalHash := crypto.Keccak256Hash(prefixedMessage)
	signatureBytes := common.FromHex(signature)
	if signatureBytes[64] >= 27 {
		signatureBytes[64] -= 27
	}

	publicKey, err := crypto.SigToPub(finalHash.Bytes(), signatureBytes)
	if err != nil {
		return ""
	}

	recoveredAddr := crypto.PubkeyToAddress(*publicKey)
	walletLower := strings.ToLower(recoveredAddr.Hex())

	return walletLower
}

// Checks if authedWallet is authorized to act on behalf of userId
func (app *ApiServer) isAuthorizedRequest(ctx context.Context, userId int32, authedWallet string) bool {

	// tests can opt in to skipAuthCheck
	if app.skipAuthCheck {
		return true
	}

	if authedWallet == "" {
		return false
	}

	cacheKey := fmt.Sprintf("%d:%s", userId, authedWallet)
	if hit, ok := app.resolveGrantCache.Get(cacheKey); ok {
		return hit
	}

	var isAuthorized bool
	err := app.pool.QueryRow(ctx, `
		SELECT EXISTS (
			-- I am the user
			SELECT 1 FROM users
			WHERE
				user_id = $1
				AND wallet = $2
				AND is_current = true
    ) OR EXISTS (
			-- I have a grant to the user
			SELECT 1 FROM grants
			WHERE
				is_current = true
				AND user_id = $1
				AND grantee_address = $2
				AND is_approved = true
				AND is_revoked = false
		);
		`, userId, authedWallet).Scan(&isAuthorized)

	if err != nil {
		return false
	}

	app.resolveGrantCache.Set(cacheKey, isAuthorized)
	return isAuthorized
}

// get the wallet that signed the request
func (app *ApiServer) getAuthedWallet(c *fiber.Ctx) string {
	return c.Locals("authedWallet").(string)
}

// get the wallet that signed the request, or "" if not set
func (app *ApiServer) tryGetAuthedWallet(c *fiber.Ctx) string {
	if c == nil {
		return ""
	}
	if w, ok := c.Locals("authedWallet").(string); ok {
		return w
	}
	return ""
}

// validateOAuthJWTTokenToUserId validates the OAuth JWT and returns the userId from the payload.
func (app *ApiServer) validateOAuthJWTTokenToUserId(ctx context.Context, token string) (trashid.HashId, error) {
	tokenParts := strings.Split(token, ".")
	if len(tokenParts) != 3 {
		return 0, fiber.NewError(fiber.StatusBadRequest, "Invalid JWT token format")
	}

	base64Header := tokenParts[0]
	base64Payload := tokenParts[1]
	base64Signature := tokenParts[2]

	paddedSignature := base64Signature
	if len(paddedSignature)%4 != 0 {
		paddedSignature += strings.Repeat("=", 4-len(paddedSignature)%4)
	}
	signatureDecoded, err := base64.URLEncoding.DecodeString(paddedSignature)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "The JWT signature could not be decoded")
	}
	signatureHex := string(signatureDecoded)
	signatureBytes := common.FromHex(signatureHex)

	message := fmt.Sprintf("%s.%s", base64Header, base64Payload)
	encodedToRecover := []byte(message)
	prefixedMessage := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(encodedToRecover), encodedToRecover))
	finalHash := crypto.Keccak256Hash(prefixedMessage)

	if len(signatureBytes) != 65 {
		return 0, fiber.NewError(fiber.StatusBadRequest, "The JWT signature was incorrectly signed")
	}
	if signatureBytes[64] >= 27 {
		signatureBytes[64] -= 27
	}
	publicKey, err := crypto.SigToPub(finalHash.Bytes(), signatureBytes)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusUnauthorized, "The JWT signature is invalid")
	}
	recoveredAddr := crypto.PubkeyToAddress(*publicKey)
	walletLower := strings.ToLower(recoveredAddr.Hex())

	paddedPayload := base64Payload
	if len(paddedPayload)%4 != 0 {
		paddedPayload += strings.Repeat("=", 4-len(paddedPayload)%4)
	}
	stringifiedPayload, err := base64.URLEncoding.DecodeString(paddedPayload)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "JWT payload could not be decoded")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(stringifiedPayload, &payload); err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "JWT payload could not be unmarshalled")
	}

	userIdInterface, exists := payload["userId"]
	if !exists {
		return 0, fiber.NewError(fiber.StatusBadRequest, "JWT payload missing userId field")
	}
	userIdStr, ok := userIdInterface.(string)
	if !ok {
		return 0, fiber.NewError(fiber.StatusBadRequest, "JWT payload userId must be a string")
	}
	jwtUserId, err := trashid.DecodeHashId(userIdStr)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "Invalid JWT payload userId")
	}

	walletUserId, err := app.queries.GetUserForWallet(ctx, walletLower)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, fiber.NewError(fiber.StatusUnauthorized, "The JWT signature is invalid - invalid wallet")
		}
		return 0, err
	}

	if int32(walletUserId) != int32(jwtUserId) {
		isManager, err := app.isActiveManager(ctx, int32(jwtUserId), int32(walletUserId))
		if err != nil {
			return 0, err
		}
		if !isManager {
			return 0, fiber.NewError(fiber.StatusForbidden, "The JWT signature is invalid - the wallet does not match the user")
		}
	}

	return trashid.HashId(jwtUserId), nil
}

// validateOAuthJWTTokenToWalletAndUserId validates the OAuth JWT and returns (wallet, userId).
// Used by auth middleware when Bearer token is not an api_access_key.
func (app *ApiServer) validateOAuthJWTTokenToWalletAndUserId(ctx context.Context, token string) (wallet string, userId int32, err error) {
	id, err := app.validateOAuthJWTTokenToUserId(ctx, token)
	if err != nil {
		return "", 0, err
	}
	tokenParts := strings.Split(token, ".")
	if len(tokenParts) != 3 {
		return "", 0, fiber.NewError(fiber.StatusBadRequest, "Invalid JWT token format")
	}
	base64Payload, base64Signature := tokenParts[1], tokenParts[2]
	paddedSignature := base64Signature
	if len(paddedSignature)%4 != 0 {
		paddedSignature += strings.Repeat("=", 4-len(paddedSignature)%4)
	}
	signatureDecoded, err := base64.URLEncoding.DecodeString(paddedSignature)
	if err != nil {
		return "", 0, fiber.NewError(fiber.StatusBadRequest, "The JWT signature could not be decoded")
	}
	signatureBytes := common.FromHex(string(signatureDecoded))
	if len(signatureBytes) != 65 {
		return "", 0, fiber.NewError(fiber.StatusBadRequest, "The JWT signature was incorrectly signed")
	}
	if signatureBytes[64] >= 27 {
		signatureBytes[64] -= 27
	}
	message := fmt.Sprintf("%s.%s", tokenParts[0], base64Payload)
	prefixedMessage := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message))
	finalHash := crypto.Keccak256Hash(prefixedMessage)
	publicKey, err := crypto.SigToPub(finalHash.Bytes(), signatureBytes)
	if err != nil {
		return "", 0, fiber.NewError(fiber.StatusUnauthorized, "The JWT signature is invalid")
	}
	recoveredAddr := crypto.PubkeyToAddress(*publicKey)
	return strings.ToLower(recoveredAddr.Hex()), int32(id), nil
}

// Middleware to set authedUserId and authedWallet in context
// Returns a 403 if either
// - the user is not authorized to act on behalf of "myId"
// - the user is not authorized to act on behalf of "myWallet"
func (app *ApiServer) authMiddleware(c *fiber.Ctx) error {
	var wallet string

	signer, _ := app.getApiSigner(c)
	myId := app.getMyId(c)
	if signer != nil {
		wallet = strings.ToLower(signer.Address)
	} else {
		wallet = app.recoverAuthorityFromSignatureHeaders(c)
		// OAuth JWT fallback: when Bearer token is not api_access_key, try as OAuth JWT (Plans app)
		if wallet == "" && myId != 0 {
			if authHeader := c.Get("Authorization"); authHeader != "" && strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
				if token != "" {
					if oauthWallet, jwtUserId, err := app.validateOAuthJWTTokenToWalletAndUserId(c.Context(), token); err == nil {
						if int32(jwtUserId) == myId {
							wallet = oauthWallet
						}
					}
				}
			}
		}
	}

	c.Locals("authedWallet", wallet)

	// Not authorized to act on behalf of myId
	if myId != 0 && !app.isAuthorizedRequest(c.Context(), myId, wallet) {
		return fiber.NewError(
			fiber.StatusForbidden,
			fmt.Sprintf(
				"You are not authorized to make this request authedWallet=%s myId=%d",
				wallet,
				myId,
			),
		)
	}

	// Not authorized to act on behalf of myWallet
	myWallet := c.Params("wallet")
	if myWallet != "" && !strings.EqualFold(myWallet, wallet) {
		return fiber.NewError(
			fiber.StatusForbidden,
			fmt.Sprintf(
				"You are not authorized to make this request authedWallet=%s myWallet=%s",
				wallet,
				myWallet,
			),
		)
	}

	return c.Next()
}

// Middleware to require auth for the userId in the route params
// Returns a 403 if the authedWallet is not authorized to act on behalf of the userId
// Should be placed after authMiddleware
func (app *ApiServer) requireAuthForUserId(c *fiber.Ctx) error {
	wallet := c.Locals("authedWallet").(string)
	userId := app.getUserId(c)
	if !app.isAuthorizedRequest(c.Context(), userId, wallet) {
		return fiber.NewError(
			fiber.StatusForbidden,
			fmt.Sprintf(
				"You are not authorized to make this request authedWallet=%s userId=%d",
				wallet,
				userId,
			),
		)
	}

	return c.Next()
}

// Middleware that asserts that there is an authed wallet
func (app *ApiServer) requireAuthMiddleware(c *fiber.Ctx) error {
	authedWallet := app.getAuthedWallet(c)
	if authedWallet == "" {
		return fiber.NewError(fiber.StatusUnauthorized, "You must be logged in to make this request")
	}

	return c.Next()
}

// Get a user from their wallet address.
//
// Note: Do NOT use this with `getAuthedWallet()` to infer the current user.
// Comms/chats _does_ use this to infer current user due to legacy reasons, as
// it predates manager mode and messages are e2ee via wallet. V1 endpoints
// should use the query or route parameters for determining the current user.
func (app *ApiServer) getUserIDFromWallet(ctx context.Context, wallet string) (int, error) {
	key := utils.CopyString(wallet)
	if hit, ok := app.resolveWalletCache.Get(key); ok {
		return hit, nil
	}

	sql := `
	SELECT user_id
	FROM users
	WHERE is_current = true
		AND handle IS NOT NULL
		AND is_available = true
		AND is_deactivated = false
		AND wallet = LOWER(@wallet)
	ORDER BY user_id ASC;
	`
	row := app.pool.QueryRow(ctx, sql, pgx.NamedArgs{
		"wallet": wallet,
	})

	userId := 0
	err := row.Scan(&userId)
	if err != nil {
		return 0, fiber.NewError(fiber.ErrBadRequest.Code, "bad signature")
	}

	app.resolveWalletCache.Set(key, userId)
	return userId, nil
}

/*
* Parses query string for a signed comms GET request and returns the userId
associated with the signing wallet
*/
func (app *ApiServer) userIdForSignedCommsRequest(c *fiber.Ctx) (int, error) {
	if c.Method() != "GET" {
		return 0, fiber.NewError(fiber.StatusBadRequest, "readSignedGet: bad method: "+c.Method())
	}

	sigBase64 := c.Get(comms.SigHeader)

	// for websocket request, read from query param instead of header
	if querySig := c.Query("signature"); sigBase64 == "" && querySig != "" {
		sigBase64 = querySig
	}

	// Check that timestamp is not too old
	timestamp, err := strconv.ParseInt(c.Query("timestamp"), 0, 64)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "failed to parse timestamp: "+err.Error())
	}

	tsAge := time.Now().UnixMilli() - timestamp
	if tsAge < 0 {
		tsAge *= -1
	}
	if tsAge > comms.SignatureTimeToLiveMs {
		return 0, fiber.NewError(fiber.StatusBadRequest, "timestamp not current")
	}

	// Strip out app_name,api_key,signature to get the parameters that are actually used to generate the signature
	uri := c.Request().URI()
	path := string(uri.Path())
	query := string(uri.QueryString())

	queryParams, err := url.ParseQuery(query)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "failed to parse query parameters: "+err.Error())
	}

	queryParams.Del("app_name")
	queryParams.Del("api_key")
	queryParams.Del("signature")

	// Build the final URL string
	urlStr := path
	if len(queryParams) > 0 {
		urlStr += "?" + queryParams.Encode()
	}

	payload := []byte(urlStr)

	wallet, pubkey, err := comms.RecoverSigningWallet(sigBase64, payload)
	if err != nil {
		return 0, fiber.NewError(fiber.StatusBadRequest, "failed to recoverSigningWallet: "+err.Error())
	}
	userId, err := app.getUserIDFromWallet(c.Context(), wallet)
	if err != nil {
		return 0, err
	}

	app.commsRpcProcessor.SetPubkeyForUser(int32(userId), pubkey)

	return userId, nil
}

func (app *ApiServer) readSignedCommsPostRequest(c *fiber.Ctx) ([]byte, string, int, error) {
	if c.Method() != "POST" {
		return nil, "", 0, fiber.NewError(fiber.StatusBadRequest, "readSignedPost bad method: "+c.Method())
	}

	payload := c.Body()

	sigHex := c.Get(comms.SigHeader)
	wallet, pubkey, err := comms.RecoverSigningWallet(sigHex, payload)
	if err != nil {
		return nil, "", 0, err
	}
	userId, err := app.getUserIDFromWallet(c.Context(), wallet)
	if err != nil {
		return nil, "", 0, err
	}

	app.commsRpcProcessor.SetPubkeyForUser(int32(userId), pubkey)
	return payload, wallet, userId, nil
}
