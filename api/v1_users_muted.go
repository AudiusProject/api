package api

import (
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

func (app *ApiServer) v1UsersMuted(c *fiber.Ctx) error {
	sql := `
		SELECT
			muted_user_id
		FROM
			muted_users
		WHERE
			user_id = @userId
			AND is_delete = FALSE
	;`

	return app.queryUsers(c, sql, pgx.NamedArgs{
		"userId": app.getUserId(c),
	})
}

func (app *ApiServer) postV1UserMute(c *fiber.Ctx) error {
	userID := app.getUserId(c)
	mutedUserID, err := trashid.DecodeHashId(c.Params("userId"))
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
		EntityId:   int64(mutedUserID),
		Action:     indexer.Action_Mute,
		EntityType: indexer.Entity_User,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   "",
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send user mute transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to mute user",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}

func (app *ApiServer) deleteV1UserMute(c *fiber.Ctx) error {
	userID := app.getUserId(c)
	mutedUserID, err := trashid.DecodeHashId(c.Params("userId"))
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
		EntityId:   int64(mutedUserID),
		Action:     indexer.Action_Unmute,
		EntityType: indexer.Entity_User,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   "",
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send user unmute transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to unmute user",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}
