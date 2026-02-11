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

func (app *ApiServer) v1UsersFollowers(c *fiber.Ctx) error {

	sql := `
	SELECT follower_user_id
	FROM follows
	JOIN users on follower_user_id = users.user_id
	JOIN aggregate_user using (user_id)
	WHERE followee_user_id = @userId
	  AND is_delete = false
	  AND is_deactivated = false
	ORDER BY follower_count desc
	LIMIT @limit
	OFFSET @offset
	`

	userId := app.getUserId(c)
	res := app.queryUsers(c, sql, pgx.NamedArgs{
		"userId": userId,
	})
	return res
}

func (app *ApiServer) postV1UserFollow(c *fiber.Ctx) error {
	userID := app.getUserId(c)
	followeeUserID, err := trashid.DecodeHashId(c.Params("userId"))
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
		EntityId:   int64(followeeUserID),
		Action:     indexer.Action_Follow,
		EntityType: indexer.Entity_User,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   "",
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send user follow transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to follow user",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}

func (app *ApiServer) deleteV1UserFollow(c *fiber.Ctx) error {
	userID := app.getUserId(c)
	followeeUserID, err := trashid.DecodeHashId(c.Params("userId"))
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
		EntityId:   int64(followeeUserID),
		Action:     indexer.Action_Unfollow,
		EntityType: indexer.Entity_User,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   "",
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send user unfollow transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to unfollow user",
		})
	}

	return c.JSON(fiber.Map{
		"success":          true,
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
	})
}
