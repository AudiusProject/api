package api

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/indexer"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type CreateDeveloperAppRequest struct {
	Name         string `json:"name"`
	Description  string `json:"description"`
	ImageUrl     string `json:"imageUrl"`
	AppSignature struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	} `json:"appSignature"`
}

type UpdateDeveloperAppRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	ImageUrl    string `json:"imageUrl"`
}

func (app *ApiServer) v1DeveloperApps(c *fiber.Ctx) error {
	address := c.Params("address")
	if address == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing address parameter")
	}

	// Add the 0x prefix if it doesn't exist
	if !strings.HasPrefix(address, "0x") {
		address = "0x" + address
	}

	developerApps, err := app.queries.GetDeveloperApps(c.Context(), dbv1.GetDeveloperAppsParams{
		Address: address,
	})
	if err != nil {
		return err
	}

	if len(developerApps) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "Developer app not found")
	}

	return c.JSON(fiber.Map{
		"data": developerApps[0],
	})

}

func (app *ApiServer) postV1DeveloperApp(c *fiber.Ctx) error {
	userID := app.getMyId(c)

	var req CreateDeveloperAppRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	metadataObj := map[string]interface{}{
		"name":        req.Name,
		"description": req.Description,
		"image_url":   req.ImageUrl,
		"app_signature": map[string]interface{}{
			"message":   req.AppSignature.Message,
			"signature": req.AppSignature.Signature,
		},
	}
	metadataBytes, _ := json.Marshal(metadataObj)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   0, // Contract requires uint, but we don't actually need this field for this action
		Action:     indexer.Action_Create,
		EntityType: indexer.Entity_DeveloperApp,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send developer app create transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create developer app",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}

func (app *ApiServer) putV1DeveloperApp(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	address := c.Params("address")

	var req UpdateDeveloperAppRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	metadataObj := map[string]interface{}{
		"address":     address,
		"name":        req.Name,
		"description": req.Description,
		"image_url":   req.ImageUrl,
	}
	metadataBytes, _ := json.Marshal(metadataObj)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   0, // Contract requires uint, but we don't actually need this field for this action
		Action:     indexer.Action_Update,
		EntityType: indexer.Entity_DeveloperApp,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send developer app update transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update developer app",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}

func (app *ApiServer) deleteV1DeveloperApp(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	address := c.Params("address")

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	metadataObj := map[string]interface{}{
		"address": address,
	}
	metadataBytes, _ := json.Marshal(metadataObj)

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   0, // Contract requires uint, but we don't actually need this field for this action
		Action:     indexer.Action_Delete,
		EntityType: indexer.Entity_DeveloperApp,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadataBytes),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send developer app delete transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to delete developer app",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}
