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
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

func (app *ApiServer) v1PlaylistReposts(c *fiber.Ctx) error {
	sql := `
	SELECT user_id
	FROM reposts r
	JOIN users u using (user_id)
	JOIN aggregate_user au using (user_id)
	WHERE repost_type != 'track'
	  AND repost_item_id = @playlistId
	  AND is_delete = false
	  AND u.is_deactivated = false
	ORDER BY follower_count desc
	LIMIT @limit
	OFFSET @offset
	`

	playlistId, err := trashid.DecodeHashId(c.Params("playlistId"))
	if err != nil {
		return err
	}

	return app.queryUsers(c, sql, pgx.NamedArgs{
		"playlistId": playlistId,
	})
}

func (app *ApiServer) postV1PlaylistRepost(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	playlistID, err := trashid.DecodeHashId(c.Params("playlistId"))
	if err != nil {
		return err
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	metadata := ""
	if body := c.Body(); len(body) > 0 {
		var reqBody struct {
			IsRepostOfRepost *bool `json:"is_repost_of_repost"`
		}
		if err := json.Unmarshal(body, &reqBody); err == nil && reqBody.IsRepostOfRepost != nil {
			metadataBytes, _ := json.Marshal(map[string]bool{"is_repost_of_repost": *reqBody.IsRepostOfRepost})
			metadata = string(metadataBytes)
		}
	}

	nonce := time.Now().UnixNano()

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(playlistID),
		Action:     indexer.Action_Repost,
		EntityType: indexer.Entity_Playlist,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   metadata,
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send playlist repost transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to repost playlist",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}

func (app *ApiServer) deleteV1PlaylistRepost(c *fiber.Ctx) error {
	userID := app.getMyId(c)
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
		Action:     indexer.Action_Unrepost,
		EntityType: indexer.Entity_Playlist,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   "",
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send playlist unrepost transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to unrepost playlist",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}
