package api

import (
	"encoding/json"
	"strconv"
	"time"

	"api.audius.co/indexer"
	"api.audius.co/trashid"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

type LocationMetadata struct {
	City    string `json:"city"`
	Region  string `json:"region"`
	Country string `json:"country"`
}

type DownloadMetadata struct {
	CID  string           `json:"cid"`
	Data LocationMetadata `json:"data"`
}

func (app *ApiServer) postV1TrackDownload(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	trackID, err := trashid.DecodeHashId(c.Params("trackId"))
	if err != nil {
		return err
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	// Parse optional location metadata from request body
	var reqBody struct {
		City    string `json:"city"`
		Region  string `json:"region"`
		Country string `json:"country"`
	}

	metadata := ""
	if err := c.BodyParser(&reqBody); err == nil {
		if reqBody.City != "" || reqBody.Region != "" || reqBody.Country != "" {
			metadataObj := DownloadMetadata{
				CID: "",
				Data: LocationMetadata{
					City:    reqBody.City,
					Region:  reqBody.Region,
					Country: reqBody.Country,
				},
			}
			metadataBytes, _ := json.Marshal(metadataObj)
			metadata = string(metadataBytes)
		}
	}

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(trackID),
		Action:     indexer.Action_Download,
		EntityType: indexer.Entity_Track,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   metadata,
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send track download transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to record track download",
		})
	}

	return c.JSON(fiber.Map{
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
	})
}
