package api

import (
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"api.audius.co/indexer"
	"api.audius.co/trashid"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type DeveloperApp struct {
	Address      string         `json:"address" db:"address"`
	UserId       trashid.HashId `json:"user_id" db:"user_id"`
	Name         string         `json:"name" db:"name"`
	Description  *string        `json:"description" db:"description"`
	ImageUrl     *string        `json:"image_url" db:"image_url"`
	RedirectURIs []string       `json:"redirect_uris" db:"redirect_uris"`
}

type DeveloperAppWithMetrics struct {
	Address             string          `json:"address" db:"address"`
	UserId              trashid.HashId  `json:"user_id" db:"user_id"`
	Name                string          `json:"name" db:"name"`
	Description         *string         `json:"description" db:"description"`
	ImageUrl            *string         `json:"image_url" db:"image_url"`
	RequestCount        int64           `json:"request_count" db:"request_count"`
	RequestCountAllTime int64           `json:"request_count_all_time" db:"request_count_all_time"`
	IsLegacy            bool            `json:"is_legacy" db:"is_legacy"`
	APIAccessKeys       json.RawMessage `json:"api_access_keys" db:"api_access_keys"`
	RedirectURIs        []string        `json:"redirect_uris" db:"redirect_uris"`
}

type CreateDeveloperAppRequest struct {
	Name         string   `json:"name" validate:"required,min=1"`
	Description  *string  `json:"description,omitempty"`
	ImageUrl     *string  `json:"image_url,omitempty"`
	RedirectURIs []string `json:"redirect_uris"`
}

type UpdateDeveloperAppRequest struct {
	Name         string   `json:"name" validate:"required,min=1"`
	Description  *string  `json:"description,omitempty"`
	ImageUrl     *string  `json:"image_url,omitempty"`
	RedirectURIs []string `json:"redirect_uris"`
}

func (app *ApiServer) v1UsersDeveloperApps(c *fiber.Ctx) error {
	userID := app.getUserId(c)
	includeMetrics := c.Query("include") == "metrics"

	if includeMetrics {
		return app.v1UsersDeveloperAppsWithMetrics(c, userID)
	}

	sql := `
		SELECT
			da.address,
			da.user_id,
			da.name,
			da.description,
			da.image_url,
			COALESCE(json_agg(oau.redirect_uri ORDER BY oau.id) FILTER (WHERE oau.redirect_uri IS NOT NULL), '[]'::json) AS redirect_uris
		FROM developer_apps da
		LEFT JOIN oauth_redirect_uris oau ON oau.client_id = da.address
		WHERE da.user_id = @userId
		AND da.is_current = true
		AND da.is_delete = false
		GROUP BY da.address, da.user_id, da.name, da.description, da.image_url
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": userID,
	})
	if err != nil {
		return err
	}

	apps, err := pgx.CollectRows(rows, pgx.RowToStructByName[DeveloperApp])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": apps,
	})
}

func (app *ApiServer) v1UsersDeveloperAppsWithMetrics(c *fiber.Ctx, userId int32) error {
	sql := `
		SELECT
			da.address,
			da.user_id,
			da.name,
			da.description,
			da.image_url,
			COALESCE(SUM(ama.request_count) FILTER (WHERE ama.date >= DATE_TRUNC('month', CURRENT_DATE)::date AND ama.date <= CURRENT_DATE), 0)::bigint AS request_count,
			COALESCE(SUM(ama.request_count), 0)::bigint AS request_count_all_time,
			NOT EXISTS (
				SELECT 1 FROM api_access_keys aak
				WHERE LOWER(aak.api_key) = LOWER(da.address) AND aak.is_active = true
			) AS is_legacy,
			COALESCE(
				(SELECT json_agg(json_build_object('api_access_key', aak.api_access_key, 'is_active', aak.is_active))
				 FROM api_access_keys aak
				 WHERE LOWER(aak.api_key) = LOWER(da.address) AND aak.is_active = true),
				'[]'::json
			) AS api_access_keys,
			oau.redirect_uris
		FROM developer_apps da
		LEFT JOIN api_metrics_apps ama ON LOWER(ama.api_key) = LOWER(da.address)
		LEFT JOIN LATERAL (
			SELECT COALESCE(json_agg(redirect_uri ORDER BY id), '[]'::json) AS redirect_uris
			FROM oauth_redirect_uris
			WHERE client_id = da.address
		) oau ON true
		WHERE da.user_id = @userId
			AND da.is_current = true
			AND da.is_delete = false
		GROUP BY da.address, da.user_id, da.name, da.description, da.image_url, oau.redirect_uris
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": userId,
	})
	if err != nil {
		return err
	}

	apps, err := pgx.CollectRows(rows, pgx.RowToStructByName[DeveloperAppWithMetrics])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": apps,
	})
}

func (app *ApiServer) putV1UsersDeveloperApp(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	if userID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id query parameter is required",
		})
	}
	address := c.Params("address")
	if address == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "address is required",
		})
	}
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}
	address = strings.ToLower(address)

	var req UpdateDeveloperAppRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}
	if err := app.requestValidator.Validate(&req); err != nil {
		return err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name is required",
		})
	}

	// Verify the app belongs to this user
	var ownerUserID int32
	err := app.pool.QueryRow(c.Context(), `
		SELECT user_id FROM developer_apps
		WHERE LOWER(address) = LOWER($1)
		  AND is_current = true
		  AND is_delete = false
		ORDER BY created_at DESC
		LIMIT 1
	`, address).Scan(&ownerUserID)
	if err != nil || ownerUserID != userID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Developer app not found",
		})
	}

	plansSecret := app.config.AudiusApiSecret
	if plansSecret == "" {
		app.logger.Error("audiusApiSecret required for developer app update")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Plans app not configured",
		})
	}
	plansKey, err := crypto.HexToECDSA(strings.TrimPrefix(plansSecret, "0x"))
	if err != nil {
		app.logger.Error("Invalid audiusApiSecret", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Plans app misconfigured",
		})
	}
	plansAddress := strings.ToLower(crypto.PubkeyToAddress(plansKey.PublicKey).Hex())

	metadataObj := map[string]interface{}{
		"address":     address,
		"name":        name,
		"description": req.Description,
		"image_url":   req.ImageUrl,
	}
	metadataBytes, _ := json.Marshal(metadataObj)

	nonce := time.Now().UnixNano()
	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(plansAddress).String(),
		UserId:     int64(userID),
		EntityId:   0,
		Action:     indexer.Action_Update,
		EntityType: indexer.Entity_DeveloperApp,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, plansKey)
	if err != nil {
		app.logger.Error("Failed to send developer app update transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update developer app",
		})
	}

	// Persist redirect URIs: replace existing set
	if app.writePool != nil {
		_, err = app.writePool.Exec(c.Context(), `DELETE FROM oauth_redirect_uris WHERE client_id = $1`, address)
		if err != nil {
			app.logger.Error("Failed to clear redirect URIs", zap.Error(err))
		}
		for _, uri := range req.RedirectURIs {
			_, err = app.writePool.Exec(c.Context(), `INSERT INTO oauth_redirect_uris (client_id, redirect_uri) VALUES ($1, $2)`, address, uri)
			if err != nil {
				app.logger.Error("Failed to insert redirect URI", zap.Error(err))
			}
		}
	}

	return c.JSON(fiber.Map{
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
	})
}

func (app *ApiServer) postV1UsersDeveloperApp(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	if userID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id query parameter is required",
		})
	}

	var req CreateDeveloperAppRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body: " + err.Error(),
		})
	}
	if err := app.requestValidator.Validate(&req); err != nil {
		return err
	}

	name := strings.TrimSpace(req.Name)
	if name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name is required",
		})
	}

	if app.writePool == nil {
		app.logger.Error("Write pool not configured")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database write not available",
		})
	}

	// Generate ECDSA keypair for the new app
	privateKey, err := ecdsa.GenerateKey(crypto.S256(), rand.Reader)
	if err != nil {
		app.logger.Error("Failed to generate keypair", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create developer app",
		})
	}
	address := strings.ToLower(crypto.PubkeyToAddress(privateKey.PublicKey).Hex())
	apiSecretHex := hex.EncodeToString(privateKey.D.Bytes())

	// Insert into api_keys
	_, err = app.writePool.Exec(c.Context(), `
		INSERT INTO api_keys (api_key, api_secret, rps, rpm)
		VALUES ($1, $2, 10, 500000)
		ON CONFLICT (api_key) DO UPDATE SET api_secret = EXCLUDED.api_secret
	`, address, apiSecretHex)
	if err != nil {
		app.logger.Error("Failed to insert api_keys", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create developer app",
		})
	}

	// Build app_signature for ManageEntity (Ethereum personal sign format so indexer can recover address)
	unixTs := strconv.FormatInt(time.Now().Unix(), 10)
	message := "Creating Audius developer app at " + unixTs
	prefixedMessage := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message))
	hash := crypto.Keccak256Hash(prefixedMessage)
	signature, err := crypto.Sign(hash.Bytes(), privateKey)
	if err != nil {
		app.logger.Error("Failed to sign app message", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create developer app",
		})
	}
	signatureHex := hex.EncodeToString(signature)

	metadataObj := map[string]interface{}{
		"address":     strings.ToLower(address),
		"name":        name,
		"description": req.Description,
		"image_url":   req.ImageUrl,
		"app_signature": map[string]interface{}{
			"message":   message,
			"signature": signatureHex,
		},
	}
	metadataBytes, _ := json.Marshal(metadataObj)

	// Sign the ManageEntity tx with our api secret (which has a grant from the user).
	// The indexer's validate_signer requires the signer to be the user or an authorized grantee.
	// The app_signature in metadata proves the new app controls its address.
	plansSecret := app.config.AudiusApiSecret
	if plansSecret == "" {
		app.logger.Error("audiusApiSecret required for developer app create")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Plans app not configured",
		})
	}
	plansKey, err := crypto.HexToECDSA(strings.TrimPrefix(plansSecret, "0x"))
	if err != nil {
		app.logger.Error("Invalid audiusApiSecret", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Plans app misconfigured",
		})
	}
	plansAddress := strings.ToLower(crypto.PubkeyToAddress(plansKey.PublicKey).Hex())

	nonce := time.Now().UnixNano()
	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(plansAddress).String(),
		UserId:     int64(userID),
		EntityId:   0,
		Action:     indexer.Action_Create,
		EntityType: indexer.Entity_DeveloperApp,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, plansKey)
	if err != nil {
		app.logger.Error("Failed to send developer app create transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create developer app",
		})
	}

	// Generate api_access_key (random base64 for Basic Auth)
	apiAccessKeyBytes := make([]byte, 32)
	if _, err := rand.Read(apiAccessKeyBytes); err != nil {
		app.logger.Error("Failed to generate api_access_key", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create developer app",
		})
	}
	apiAccessKey := base64.URLEncoding.EncodeToString(apiAccessKeyBytes)

	_, err = app.writePool.Exec(c.Context(), `
		INSERT INTO api_access_keys (api_key, api_access_key, is_active)
		VALUES ($1, $2, true)
	`, address, apiAccessKey)
	if err != nil {
		app.logger.Error("Failed to insert api_access_keys", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create developer app",
		})
	}

	// Persist redirect URIs
	_, err = app.writePool.Exec(c.Context(), `DELETE FROM oauth_redirect_uris WHERE client_id = $1`, address)
	if err != nil {
		app.logger.Error("Failed to clear redirect URIs", zap.Error(err))
	}
	for _, uri := range req.RedirectURIs {
		_, err = app.writePool.Exec(c.Context(), `INSERT INTO oauth_redirect_uris (client_id, redirect_uri) VALUES ($1, $2)`, address, uri)
		if err != nil {
			app.logger.Error("Failed to insert redirect URI", zap.Error(err))
		}
	}

	return c.JSON(fiber.Map{
		"api_key":          address,
		"api_secret":       apiSecretHex,
		"bearer_token":     apiAccessKey,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
	})
}

func (app *ApiServer) deleteV1UsersDeveloperApp(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	if userID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id query parameter is required",
		})
	}
	address := c.Params("address")
	if address == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "address is required",
		})
	}
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}

	// Verify the app belongs to this user
	var ownerUserID int32
	err := app.pool.QueryRow(c.Context(), `
		SELECT user_id FROM developer_apps
		WHERE LOWER(address) = LOWER($1)
		  AND is_current = true
		  AND is_delete = false
		ORDER BY created_at DESC
		LIMIT 1
	`, address).Scan(&ownerUserID)
	if err != nil || ownerUserID != userID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Developer app not found",
		})
	}

	plansSecret := app.config.AudiusApiSecret
	if plansSecret == "" {
		app.logger.Error("audiusApiSecret required for developer app delete")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Plans app not configured",
		})
	}
	plansKey, err := crypto.HexToECDSA(strings.TrimPrefix(plansSecret, "0x"))
	if err != nil {
		app.logger.Error("Invalid audiusApiSecret", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Plans app misconfigured",
		})
	}
	plansAddress := strings.ToLower(crypto.PubkeyToAddress(plansKey.PublicKey).Hex())

	// 1. Delete api_access_keys (revoke all access keys for this app)
	// 2. Delete api_keys row
	// 3. Send ManageEntity transaction to delete the developer app on-chain
	if app.writePool != nil {
		_, err = app.writePool.Exec(c.Context(), `DELETE FROM api_access_keys WHERE LOWER(api_key) = LOWER($1)`, address)
		if err != nil {
			app.logger.Error("Failed to delete api_access_keys", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to delete developer app",
			})
		}
		_, err = app.writePool.Exec(c.Context(), `DELETE FROM api_keys WHERE LOWER(api_key) = LOWER($1)`, address)
		if err != nil {
			app.logger.Error("Failed to delete api_keys", zap.Error(err))
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to delete developer app",
			})
		}
	}

	metadataObj := map[string]interface{}{
		"address": strings.ToLower(address),
	}
	metadataBytes, _ := json.Marshal(metadataObj)

	nonce := time.Now().UnixNano()
	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(plansAddress).String(),
		UserId:     int64(userID),
		EntityId:   0,
		Action:     indexer.Action_Delete,
		EntityType: indexer.Entity_DeveloperApp,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, plansKey)
	if err != nil {
		app.logger.Error("Failed to send developer app delete transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete developer app",
		})
	}

	return c.JSON(fiber.Map{
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
	})
}

type registerApiKeyBody struct {
	ApiSecret string `json:"api_secret"`
}

// postV1UsersDeveloperAppRegisterApiKey inserts api_key and api_secret into api_keys
// for developer apps created via entity manager transactions. Used when the client
// sends raw ManageEntity tx instead of POST /developer-apps. Inserts with rps=10,
// rpm=500000 (same as POST create). Requires the app to exist in developer_apps and
// belong to the authenticated user.
func (app *ApiServer) postV1UsersDeveloperAppRegisterApiKey(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	if userID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id query parameter is required",
		})
	}

	address := c.Params("address")
	if address == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "address is required",
		})
	}
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}
	apiKey := strings.ToLower(address)

	var body registerApiKeyBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	apiSecret := strings.TrimSpace(body.ApiSecret)
	if apiSecret == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "api_secret is required",
		})
	}

	// Verify the app belongs to this user
	var ownerUserID int32
	err := app.pool.QueryRow(c.Context(), `
		SELECT user_id FROM developer_apps
		WHERE LOWER(address) = LOWER($1)
		  AND is_current = true
		  AND is_delete = false
		ORDER BY created_at DESC
		LIMIT 1
	`, apiKey).Scan(&ownerUserID)
	if err != nil || ownerUserID != userID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Developer app not found",
		})
	}

	if app.writePool == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database write not available",
		})
	}

	_, err = app.writePool.Exec(c.Context(), `
		INSERT INTO api_keys (api_key, api_secret, rps, rpm)
		VALUES ($1, $2, 10, 500000)
		ON CONFLICT (api_key) DO UPDATE SET api_secret = EXCLUDED.api_secret
	`, apiKey, apiSecret)
	if err != nil {
		app.logger.Error("Failed to insert api_keys", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to register API key",
		})
	}

	return c.JSON(fiber.Map{"success": true})
}

type deactivateAccessKeyBody struct {
	ApiAccessKey string `json:"api_access_key"`
}

func (app *ApiServer) postV1UsersDeveloperAppAccessKeyDeactivate(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	if userID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id query parameter is required",
		})
	}
	address := c.Params("address")
	if address == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "address is required",
		})
	}
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}

	var body deactivateAccessKeyBody
	if err := c.BodyParser(&body); err != nil || strings.TrimSpace(body.ApiAccessKey) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "api_access_key is required",
		})
	}
	apiAccessKey := strings.TrimSpace(body.ApiAccessKey)

	// Verify the app belongs to this user
	var ownerUserID int32
	err := app.pool.QueryRow(c.Context(), `
		SELECT user_id FROM developer_apps
		WHERE LOWER(address) = LOWER($1)
		  AND is_current = true
		  AND is_delete = false
		ORDER BY created_at DESC
		LIMIT 1
	`, address).Scan(&ownerUserID)
	if err != nil || ownerUserID != userID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Developer app not found",
		})
	}

	if app.writePool == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database write not available",
		})
	}

	result, err := app.writePool.Exec(c.Context(), `
		UPDATE api_access_keys
		SET is_active = false
		WHERE LOWER(api_key) = LOWER($1) AND api_access_key = $2
	`, address, apiAccessKey)
	if err != nil {
		app.logger.Error("Failed to deactivate api_access_key", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to deactivate access key",
		})
	}
	if result.RowsAffected() == 0 {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Access key not found",
		})
	}

	// Invalidate signer cache so deactivated key is no longer accepted
	app.apiAccessKeySignerCache.Delete(apiAccessKey)

	return c.JSON(fiber.Map{"success": true})
}

func (app *ApiServer) postV1UsersDeveloperAppAccessKey(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	if userID == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id query parameter is required",
		})
	}
	address := c.Params("address")
	if address == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "address is required",
		})
	}
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}
	address = strings.ToLower(address)

	// Verify the app belongs to this user
	var ownerUserID int32
	err := app.pool.QueryRow(c.Context(), `
		SELECT user_id FROM developer_apps
		WHERE LOWER(address) = LOWER($1)
		  AND is_current = true
		  AND is_delete = false
		ORDER BY created_at DESC
		LIMIT 1
	`, address).Scan(&ownerUserID)
	if err != nil || ownerUserID != userID {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Developer app not found",
		})
	}

	if app.writePool == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Database write not available",
		})
	}

	apiAccessKeyBytes := make([]byte, 32)
	if _, err := rand.Read(apiAccessKeyBytes); err != nil {
		app.logger.Error("Failed to generate api_access_key", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create access key",
		})
	}
	apiAccessKey := base64.URLEncoding.EncodeToString(apiAccessKeyBytes)

	_, err = app.writePool.Exec(c.Context(), `
		INSERT INTO api_access_keys (api_key, api_access_key, is_active)
		VALUES ($1, $2, true)
	`, address, apiAccessKey)
	if err != nil {
		app.logger.Error("Failed to insert api_access_keys", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create access key",
		})
	}

	return c.JSON(fiber.Map{"api_access_key": apiAccessKey})
}
