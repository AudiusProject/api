package api

import (
	"encoding/json"
	"errors"

	comms "bridgerton.audius.co/api/comms"

	"github.com/AudiusProject/audiusd/pkg/logger"
	"github.com/gofiber/fiber/v2"
)

/* TODO List
- Decide if we want to validate first and then apply or validate inside apply within a transaction
- Attach instance of rpc processor or validator to the server?
*/

func (app *ApiServer) mutateChat(c *fiber.Ctx) error {
	payload, wallet, err := comms.ReadSignedPost(c)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad request: "+err.Error())
	}

	// unmarshal RPC and call validator
	var rawRpc comms.RawRPC
	err = json.Unmarshal(payload, &rawRpc)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad request: "+err.Error())
	}

	//
	// rpcLog := &schema.RpcLog{
	// 	RelayedBy:  s.config.MyHost,
	// 	RelayedAt:  time.Now(),
	// 	FromWallet: wallet,
	// 	Rpc:        payload,
	// 	Sig:        c.Request().Header.Get(signing.SigHeader),
	// }

	// authedWallet := app.getAuthedWallet(c)
	userId, err := app.getUserIDFromWallet(c.Context(), wallet)
	if err != nil {
		return err
	}

	// userId, err := rpcz.GetRPCCurrentUserID(rpcLog, &rawRpc)
	// if err != nil {
	// 	return c.String(400, "wallet not found: "+err.Error())
	// }

	// call validator
	err = s.proc.Validate(userId, rawRpc)
	if err != nil {
		if errors.Is(err, rpcz.ErrAttestationFailed) {
			return c.JSON(403, "bad request: "+err.Error())
		}
		return c.JSON(400, "bad request: "+err.Error())
	}

	ok, err := s.proc.Apply(rpcLog)
	if err != nil {
		logger.Warn(string(payload), "wallet", wallet, "err", err)
		return err
	}
	logger.Debug(string(payload), "wallet", wallet, "relay", true)
	return c.JSON(200, ok)
}
