package api

import (
	"encoding/json"
	"errors"
	"time"

	comms "bridgerton.audius.co/api/comms"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func (app *ApiServer) mutateChat(c *fiber.Ctx) error {
	payload, wallet, userId, err := app.readSignedCommsPostRequest(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad request: "+err.Error())
	}

	// unmarshal RPC and call validator
	var rawRpc comms.RawRPC
	err = json.Unmarshal(payload, &rawRpc)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad request: "+err.Error())
	}

	rpcLog := &comms.RpcLog{
		RelayedBy:  "bridge",
		RelayedAt:  time.Now(),
		FromWallet: wallet,
		Rpc:        payload,
		Sig:        c.Get(comms.SigHeader),
	}

	err = app.commsRpcProcessor.Validate(c.Context(), int32(userId), rawRpc)
	if err != nil {
		if errors.Is(err, comms.ErrAttestationFailed) {
			return fiber.NewError(
				fiber.StatusForbidden,
				"forbidden to make this request: "+err.Error(),
			)
		}
		return fiber.NewError(
			fiber.StatusBadRequest,
			"bad request: "+err.Error(),
		)
	}

	err = app.commsRpcProcessor.Apply(c.Context(), rpcLog)
	if err != nil {
		app.logger.Warn("comms rpc apply failed", zap.String("payload", string(payload)), zap.String("wallet", wallet), zap.Error(err))
		return err
	}
	app.logger.Debug("comms rpc apply succeeded", zap.String("payload", string(payload)), zap.String("wallet", wallet), zap.Bool("relay", true))
	return c.JSON(rpcLog)
}
