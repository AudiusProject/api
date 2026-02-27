package api

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/indexer"
	"api.audius.co/trashid"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type createGrantBody struct {
	AppApiKey string `json:"app_api_key"`
}

type addManagerBody struct {
	ManagerUserId string `json:"manager_user_id"`
}

type approveGrantBody struct {
	GrantorUserId string `json:"grantor_user_id"`
}

// postV1UsersGrant creates a grant from the user to an app (user authorizes app to act on their behalf)
func (app *ApiServer) postV1UsersGrant(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	pathUserID, err := trashid.DecodeHashId(c.Params("userId"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid user ID")
	}
	if int32(pathUserID) != userID {
		return fiber.NewError(fiber.StatusForbidden, "User ID does not match authenticated user")
	}

	var body createGrantBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	appApiKey := strings.TrimSpace(body.AppApiKey)
	if appApiKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "app_api_key is required",
		})
	}
	if !strings.HasPrefix(strings.ToLower(appApiKey), "0x") {
		appApiKey = "0x" + appApiKey
	}
	appApiKey = strings.ToLower(appApiKey)

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()
	metadata := map[string]string{"grantee_address": appApiKey}
	metadataBytes, _ := json.Marshal(metadata)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   0,
		Action:     indexer.Action_Create,
		EntityType: indexer.Entity_Grant,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send create grant transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create grant",
		})
	}

	return c.JSON(fiber.Map{
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
	})
}

// deleteV1UsersGrant revokes a grant from the user to an app
func (app *ApiServer) deleteV1UsersGrant(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	pathUserID, err := trashid.DecodeHashId(c.Params("userId"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid user ID")
	}
	if int32(pathUserID) != userID {
		return fiber.NewError(fiber.StatusForbidden, "User ID does not match authenticated user")
	}

	appApiKey := c.Params("address")
	if appApiKey == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "address (app API key) is required",
		})
	}
	if !strings.HasPrefix(strings.ToLower(appApiKey), "0x") {
		appApiKey = "0x" + appApiKey
	}
	appApiKey = strings.ToLower(appApiKey)

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()
	metadata := map[string]string{"grantee_address": appApiKey}
	metadataBytes, _ := json.Marshal(metadata)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   0,
		Action:     indexer.Action_Delete,
		EntityType: indexer.Entity_Grant,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send revoke grant transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to revoke grant",
		})
	}

	return c.JSON(fiber.Map{
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
	})
}

// postV1UsersManager adds a manager (user-to-user grant). The child user adds another user as manager.
func (app *ApiServer) postV1UsersManager(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	pathUserID, err := trashid.DecodeHashId(c.Params("userId"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid user ID")
	}
	if int32(pathUserID) != userID {
		return fiber.NewError(fiber.StatusForbidden, "User ID does not match authenticated user")
	}

	var body addManagerBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	managerUserID, err := trashid.DecodeHashId(body.ManagerUserId)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid manager_user_id",
		})
	}

	// Get manager's wallet (grantee_address)
	users, err := app.queries.Users(c.Context(), dbv1.GetUsersParams{
		MyID: 0,
		Ids:  []int32{int32(managerUserID)},
	})
	if err != nil || len(users) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "manager_user_id is invalid",
		})
	}
	managerWallet := users[0].Wallet
	if !managerWallet.Valid || managerWallet.String == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Manager user has no wallet",
		})
	}
	granteeAddress := strings.ToLower(managerWallet.String)
	if !strings.HasPrefix(granteeAddress, "0x") {
		granteeAddress = "0x" + granteeAddress
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()
	metadata := map[string]string{"grantee_address": granteeAddress}
	metadataBytes, _ := json.Marshal(metadata)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   0,
		Action:     indexer.Action_Create,
		EntityType: indexer.Entity_Grant,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send add manager transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to add manager",
		})
	}

	return c.JSON(fiber.Map{
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
	})
}

// deleteV1UsersManager removes a manager. Can be called by the child user or the manager.
func (app *ApiServer) deleteV1UsersManager(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	pathUserID, err := trashid.DecodeHashId(c.Params("userId"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid user ID")
	}
	managerUserID, err := trashid.DecodeHashId(c.Params("managerUserId"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid manager user ID")
	}

	// Either the child user (userId) or the manager (managerUserId) can remove
	if int32(pathUserID) != userID && int32(managerUserID) != userID {
		return fiber.NewError(fiber.StatusForbidden, "Only the user or manager can remove this grant")
	}

	// Get manager's wallet
	users, err := app.queries.Users(c.Context(), dbv1.GetUsersParams{
		MyID: 0,
		Ids:  []int32{int32(managerUserID)},
	})
	if err != nil || len(users) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "manager_user_id is invalid",
		})
	}
	managerWallet := users[0].Wallet
	if !managerWallet.Valid || managerWallet.String == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Manager user has no wallet",
		})
	}
	granteeAddress := strings.ToLower(managerWallet.String)
	if !strings.HasPrefix(granteeAddress, "0x") {
		granteeAddress = "0x" + granteeAddress
	}

	// The grantor is the child user (path userId)
	grantorUserID := int32(pathUserID)

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()
	metadata := map[string]string{"grantee_address": granteeAddress}
	metadataBytes, _ := json.Marshal(metadata)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(grantorUserID),
		EntityId:   0,
		Action:     indexer.Action_Delete,
		EntityType: indexer.Entity_Grant,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send remove manager transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to remove manager",
		})
	}

	return c.JSON(fiber.Map{
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
	})
}

// postV1UsersApproveGrant approves a manager request. The manager (grantee) approves being added by the child (grantor).
func (app *ApiServer) postV1UsersApproveGrant(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	pathUserID, err := trashid.DecodeHashId(c.Params("userId"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid user ID")
	}
	if int32(pathUserID) != userID {
		return fiber.NewError(fiber.StatusForbidden, "User ID does not match authenticated user")
	}

	var body approveGrantBody
	if err := c.BodyParser(&body); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}
	grantorUserID, err := trashid.DecodeHashId(body.GrantorUserId)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid grantor_user_id",
		})
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()
	metadata := map[string]interface{}{"grantor_user_id": int64(grantorUserID)}
	metadataBytes, _ := json.Marshal(metadata)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   0,
		Action:     indexer.Action_Approve,
		EntityType: indexer.Entity_Grant,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send approve grant transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to approve grant",
		})
	}

	return c.JSON(fiber.Map{
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
	})
}
