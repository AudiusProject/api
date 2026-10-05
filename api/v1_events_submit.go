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

type SubmitToContestBody struct {
	TrackID string `json:"track_id"`
}

// postV1EventSubmitToContest enters a track into an open-contest event.
// Open contests don't have a parent track (unlike remix_contest), so we
// can't infer the submission from the remixes table — discovery indexes
// the SubmitToContest action into contest_submissions instead.
func (app *ApiServer) postV1EventSubmitToContest(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	eventID, err := trashid.DecodeHashId(c.Params("eventId"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid event id")
	}

	body := SubmitToContestBody{}
	if err := c.BodyParser(&body); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid body")
	}
	if body.TrackID == "" {
		return fiber.NewError(fiber.StatusBadRequest, "track_id is required")
	}
	trackID, err := trashid.DecodeHashId(body.TrackID)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid track id")
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	metadata, err := json.Marshal(map[string]any{
		"track_id": trackID,
	})
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()
	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(eventID),
		Action:     indexer.Action_SubmitToContest,
		EntityType: indexer.Entity_Event,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   string(metadata),
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send SubmitToContest transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to submit to contest",
		})
	}

	return c.JSON(fiber.Map{
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
	})
}
