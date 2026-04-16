package api

import (
	"errors"
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

// postV1EventFollow subscribes the authenticated user to a remix-contest event
// so they'll be notified when the event's artist posts an update. It emits a
// Subscribe/Event ManageEntity transaction and relies on the indexer to write
// the subscriptions row.
func (app *ApiServer) postV1EventFollow(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	eventID, err := trashid.DecodeHashId(c.Params("eventId"))
	if err != nil {
		return err
	}

	// Sanity-check that the event actually exists before we burn a chain tx.
	var exists bool
	if err := app.pool.QueryRow(c.Context(), `
		SELECT EXISTS (
			SELECT 1 FROM events
			WHERE event_id = $1 AND COALESCE(is_deleted, false) = false
		)
	`, eventID).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return fiber.NewError(fiber.StatusNotFound, "event not found")
	}

	signer, err := app.getApiSigner(c)
	if err != nil {
		return err
	}

	nonce := time.Now().UnixNano()

	manageEntityTx := &corev1.ManageEntityLegacy{
		Signer:     common.HexToAddress(signer.Address).String(),
		UserId:     int64(userID),
		EntityId:   int64(eventID),
		Action:     indexer.Action_Subscribe,
		EntityType: indexer.Entity_Event,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   "",
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send event follow transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to follow event",
		})
	}

	return c.JSON(fiber.Map{
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
	})
}

func (app *ApiServer) deleteV1EventFollow(c *fiber.Ctx) error {
	userID := app.getMyId(c)
	eventID, err := trashid.DecodeHashId(c.Params("eventId"))
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
		EntityId:   int64(eventID),
		Action:     indexer.Action_Unsubscribe,
		EntityType: indexer.Entity_Event,
		Nonce:      strconv.FormatInt(nonce, 10),
		Metadata:   "",
	}

	response, err := app.sendTransactionWithSigner(manageEntityTx, signer.PrivateKey)
	if err != nil {
		app.logger.Error("Failed to send event unfollow transaction", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to unfollow event",
		})
	}

	return c.JSON(fiber.Map{
		"transaction_hash": response.Msg.GetTransaction().GetHash(),
		"block_hash":       response.Msg.GetTransaction().GetBlockHash(),
		"block_number":     response.Msg.GetTransaction().GetHeight(),
	})
}

// v1EventFollowState returns { is_followed, follower_count } for an event.
// Used by the contest page to render the follow button in the right state.
func (app *ApiServer) v1EventFollowState(c *fiber.Ctx) error {
	eventID, err := trashid.DecodeHashId(c.Params("eventId"))
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid event id")
	}
	myID := app.getMyId(c)

	var followerCount int
	if err := app.pool.QueryRow(c.Context(), `
		SELECT COUNT(*)
		FROM subscriptions
		WHERE entity_type = 'Event'
		  AND user_id = $1
		  AND is_current = true
		  AND is_delete = false
	`, eventID).Scan(&followerCount); err != nil {
		if !errors.Is(err, pgx.ErrNoRows) {
			return err
		}
	}

	var isFollowed bool
	if myID > 0 {
		if err := app.pool.QueryRow(c.Context(), `
			SELECT EXISTS (
				SELECT 1 FROM subscriptions
				WHERE entity_type = 'Event'
				  AND user_id = $1
				  AND subscriber_id = $2
				  AND is_current = true
				  AND is_delete = false
			)
		`, eventID, myID).Scan(&isFollowed); err != nil {
			return err
		}
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"is_followed":    isFollowed,
			"follower_count": followerCount,
		},
	})
}
