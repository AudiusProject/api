package api

import (
	"math/big"
	"sync"

	"connectrpc.com/connect"
	v1 "github.com/AudiusProject/audiusd/pkg/api/core/v1"
	"github.com/AudiusProject/audiusd/pkg/core/server"
	eth_gen "github.com/AudiusProject/audiusd/pkg/eth/contracts/gen"
	"github.com/ethereum/go-ethereum/accounts/abi"
	gcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

const (
	manageEntityFunctionName      = "manageEntity"
	manageEntityMetadataArgName   = "_metadata"
	manageEntityActionArgName     = "_action"
	manageEntityUserIdArgName     = "_userId"
	manageEntityEntityTypeArgName = "_entityType"
	manageEntityEntityIdArgName   = "_entityId"
	manageEntityNonceArgName      = "_nonce"
	manageEntitySubjectSigArgName = "_subjectSig"
)

var (
	entityManagerABI  *abi.ABI
	entityManagerOnce sync.Once
	entityManagerErr  error
)

type RelayRequest struct {
	EncodedABI string `json:"encodedABI"`
}

func DecodeManageEntityABI(encodedABI string) (*v1.ManageEntityLegacy, error) {
	entityManagerOnce.Do(func() {
		var err error
		entityManagerABI, err = eth_gen.EntityManagerMetaData.GetAbi()
		if err != nil {
			entityManagerErr = err
		}
	})
	if entityManagerErr != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "internal server error: "+entityManagerErr.Error())
	}
	data := gcommon.FromHex(encodedABI)
	method, err := entityManagerABI.MethodById(data)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: "+err.Error())
	}

	if method.Name != manageEntityFunctionName {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: expected manageEntity function")
	}

	params := make(map[string]interface{})
	err = method.Inputs.UnpackIntoMap(params, data[4:])
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: "+err.Error())
	}

	metadata, ok := params[manageEntityMetadataArgName]
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: metadata is required")
	}
	metadataStr, ok := metadata.(string)
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: metadata is not a string")
	}

	action, ok := params[manageEntityActionArgName]
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: action is required")
	}
	actionStr, ok := action.(string)
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: action is not a string")
	}

	userId, ok := params[manageEntityUserIdArgName]
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: userId is required")
	}
	userIdStr, ok := userId.(*big.Int)
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: userId is not a big.Int")
	}

	entityType, ok := params[manageEntityEntityTypeArgName]
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: entityType is required")
	}
	entityTypeStr, ok := entityType.(string)
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: entityType is not a string")
	}

	entityId, ok := params[manageEntityEntityIdArgName]
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: entityId is required")
	}
	entityIdStr, ok := entityId.(*big.Int)
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: entityId is not a big.Int")
	}

	nonce, ok := params[manageEntityNonceArgName]
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: nonce is required")
	}
	nonceStr, ok := nonce.([32]byte)
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: nonce is not a [32]byte")
	}

	subjectSig, ok := params[manageEntitySubjectSigArgName]
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: subjectSig is required")
	}
	subjectSigStr, ok := subjectSig.([]byte)
	if !ok {
		return nil, fiber.NewError(fiber.StatusBadRequest, "bad request: subjectSig is not a []byte")
	}

	return &v1.ManageEntityLegacy{
		Metadata:   metadataStr,
		Action:     actionStr,
		UserId:     userIdStr.Int64(),
		EntityType: entityTypeStr,
		EntityId:   entityIdStr.Int64(),
		Nonce:      hexutil.Encode(nonceStr[:]),
		Signature:  hexutil.Encode(subjectSigStr),
	}, nil
}

func (app *ApiServer) relay(c *fiber.Ctx) error {
	ctx := c.Context()
	logger := app.logger
	oapSdk := app.auds

	var request RelayRequest
	if err := c.BodyParser(&request); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad request: "+err.Error())
	}

	if request.EncodedABI == "" {
		return fiber.NewError(fiber.StatusBadRequest, "bad request: encodedABI is required")
	}

	decodedTx, err := DecodeManageEntityABI(request.EncodedABI)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad request: "+err.Error())
	}

	wallet, _, err := server.RecoverPubkeyFromCoreTx(nil, decodedTx)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad request: "+err.Error())
	}

	// TODO: submit pubkey to backfill pubkey queue

	logger.Info("relaying transaction", zap.String("wallet", wallet), zap.Any("decodedTx", decodedTx))

	// TODO: check if user or app is authorized to act on behalf of userId

	// if user matches userId, they are authorized
	// if app has a grant to the user, they are authorized

	// submit tx to core
	res, err := oapSdk.Core.SendTransaction(ctx, connect.NewRequest(&v1.SendTransactionRequest{
		Transaction: &v1.SignedTransaction{
			Transaction: &v1.SignedTransaction_ManageEntity{
				ManageEntity: decodedTx,
			},
		},
	}))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to send transaction: "+err.Error())
	}

	msg := res.Msg.Transaction

	receipt := &types.Receipt{
		TxHash:            gcommon.HexToHash(msg.GetHash()),
		BlockHash:         gcommon.HexToHash(msg.GetBlockHash()),
		BlockNumber:       big.NewInt(int64(msg.GetHeight())),
		TransactionIndex:  0,
		GasUsed:           10,
		CumulativeGasUsed: 10,
		EffectiveGasPrice: big.NewInt(420),
		Status:            types.ReceiptStatusSuccessful,
	}
	return c.JSON(receipt)
}
