package api

import (
	"strconv"
	"time"

	"api.audius.co/indexer"
	"api.audius.co/trashid"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (app *ApiServer) postV1PlaylistShare(c *fiber.Ctx) error {
	userID := app.getUserId(c)
	playlistID, err := trashid.DecodeHashId(c.Params("playlistId"))
	if err != nil {
		return err
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(playlistID),
		Action:     indexer.Action_Share,
		EntityType: indexer.Entity_Playlist,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   "",
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send playlist share transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to share playlist",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}
