package api

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"sync"

	"api.audius.co/config"
	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	cconfig "github.com/OpenAudio/go-openaudio/pkg/core/config"
	"github.com/OpenAudio/go-openaudio/pkg/core/server"
	eth_gen "github.com/OpenAudio/go-openaudio/pkg/eth/contracts/gen"
	"github.com/ethereum/go-ethereum/accounts/abi"
	gcommon "github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

const (
	// ABI Function Arguments
	manageEntityFunctionName      = "manageEntity"
	manageEntityMetadataArgName   = "_metadata"
	manageEntityActionArgName     = "_action"
	manageEntityUserIdArgName     = "_userId"
	manageEntityEntityTypeArgName = "_entityType"
	manageEntityEntityIdArgName   = "_entityId"
	manageEntityNonceArgName      = "_nonce"
	manageEntitySubjectSigArgName = "_subjectSig"

	// Actions
	ActionCreate   = "Create"
	ActionUpdate   = "Update"
	ActionDelete   = "Delete"
	ActionGrant    = "Grant"
	ActionRevoke   = "Revoke"
	ActionTransfer = "Transfer"
	ActionApprove  = "Approve"
	ActionReject   = "Reject"
	ActionCancel   = "Cancel"

	// Entity Types
	EntityTypeUser       = "User"
	EntityTypeApp        = "App"
	EntityTypeTrack      = "Track"
	EntityTypePlaylist   = "Playlist"
	EntityTypeAlbum      = "Album"
	EntityTypeCollection = "Collection"

	OperationCreateUser   = ActionCreate + EntityTypeUser
	OperationUpdateUser   = ActionUpdate + EntityTypeUser
	OperationDeleteUser   = ActionDelete + EntityTypeUser
	OperationGrantUser    = ActionGrant + EntityTypeUser
	OperationRevokeUser   = ActionRevoke + EntityTypeUser
	OperationTransferUser = ActionTransfer + EntityTypeUser
	OperationApproveUser  = ActionApprove + EntityTypeUser
	OperationRejectUser   = ActionReject + EntityTypeUser
	OperationCancelUser   = ActionCancel + EntityTypeUser
)

var (
	// entityManagerABI global singleton
	entityManagerABI  *abi.ABI
	entityManagerOnce sync.Once
	entityManagerErr  error

	// anonymouslyAllowedActions map that matches operation to struct{}
	anonymouslyAllowedActions = map[string]struct{}{
		OperationCreateUser: {},
	}
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
		return nil, fmt.Errorf("failed to get entity manager ABI: %w", entityManagerErr)
	}
	data := gcommon.FromHex(encodedABI)
	method, err := entityManagerABI.MethodById(data)
	if err != nil {
		return nil, fmt.Errorf("failed to get method by ID: %w", err)
	}

	if method.Name != manageEntityFunctionName {
		return nil, fmt.Errorf("expected manageEntity function, got %s", method.Name)
	}

	params := make(map[string]interface{})
	err = method.Inputs.UnpackIntoMap(params, data[4:])
	if err != nil {
		return nil, fmt.Errorf("failed to unpack method inputs: %w", err)
	}

	metadata, ok := params[manageEntityMetadataArgName]
	if !ok {
		return nil, fmt.Errorf("metadata is required")
	}
	metadataStr, ok := metadata.(string)
	if !ok {
		return nil, fmt.Errorf("metadata is not a string")
	}

	action, ok := params[manageEntityActionArgName]
	if !ok {
		return nil, fmt.Errorf("action is required")
	}
	actionStr, ok := action.(string)
	if !ok {
		return nil, fmt.Errorf("action is not a string")
	}

	userId, ok := params[manageEntityUserIdArgName]
	if !ok {
		return nil, fmt.Errorf("userId is required")
	}
	userIdStr, ok := userId.(*big.Int)
	if !ok {
		return nil, fmt.Errorf("userId is not a big.Int")
	}

	entityType, ok := params[manageEntityEntityTypeArgName]
	if !ok {
		return nil, fmt.Errorf("entityType is required")
	}
	entityTypeStr, ok := entityType.(string)
	if !ok {
		return nil, fmt.Errorf("entityType is not a string")
	}

	entityId, ok := params[manageEntityEntityIdArgName]
	if !ok {
		return nil, fmt.Errorf("entityId is required")
	}
	entityIdStr, ok := entityId.(*big.Int)
	if !ok {
		return nil, fmt.Errorf("entityId is not a big.Int")
	}

	nonce, ok := params[manageEntityNonceArgName]
	if !ok {
		return nil, fmt.Errorf("nonce is required")
	}
	nonceStr, ok := nonce.([32]byte)
	if !ok {
		return nil, fmt.Errorf("nonce is not a [32]byte")
	}

	subjectSig, ok := params[manageEntitySubjectSigArgName]
	if !ok {
		return nil, fmt.Errorf("subjectSig is required")
	}
	subjectSigStr, ok := subjectSig.([]byte)
	if !ok {
		return nil, fmt.Errorf("subjectSig is not a []byte")
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

	wallet, _, err := server.RecoverPubkeyFromCoreTx(&cconfig.Config{
		AcdcChainID:              config.Cfg.AudiusdChainID,
		AcdcEntityManagerAddress: config.Cfg.AudiusdEntityManagerAddress,
	}, decodedTx)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "bad request: "+err.Error())
	}

	sender := decodedTx.GetSigner()
	if !strings.EqualFold(sender, wallet) {
		return fiber.NewError(fiber.StatusForbidden, "forbidden: signer does not match sender")
	}

	operation := decodedTx.Action + decodedTx.EntityType
	_, anonymouslyAllowed := anonymouslyAllowedActions[operation]
	if anonymouslyAllowed {
		msg, err := app.handleRelay(ctx, decodedTx)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "failed to handle relay: "+err.Error())
		}
		receipt := transactionToReceipt(msg, wallet)
		return c.JSON(map[string]interface{}{
			"receipt": receipt,
		})
	}

	exists := false

	// query users table by wallet
	userSQL := `
		SELECT user_id
		FROM users
		WHERE is_current = true
			AND is_deactivated = false
			AND wallet = LOWER(@wallet)
		LIMIT 1
	`
	var userId int
	err = app.pool.QueryRow(ctx, userSQL, pgx.NamedArgs{
		"wallet": wallet,
	}).Scan(&userId)
	if err == nil {
		exists = true
	} else if err != pgx.ErrNoRows {
		logger.Error("error querying users table", zap.Error(err))
	}

	// query apps table by address
	if !exists {
		appSQL := `
			SELECT address
			FROM developer_apps
			WHERE is_current = true
				AND is_delete = false
				AND address = @address
			LIMIT 1
		`
		var address string
		err = app.pool.QueryRow(ctx, appSQL, pgx.NamedArgs{
			"address": wallet,
		}).Scan(&address)
		if err == nil {
			exists = true
		} else if err != pgx.ErrNoRows {
			logger.Error("error querying developer_apps table", zap.Error(err))
		}
	}

	if !exists {
		return fiber.NewError(fiber.StatusForbidden, "forbidden: wallet is not a user or app: "+wallet)
	}
	// TODO: submit pubkey to backfill pubkey queue

	logger.Info("relaying transaction", zap.String("wallet", wallet), zap.Any("decodedTx", decodedTx))
	msg, err := app.handleRelay(ctx, decodedTx)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "failed to handle relay: "+err.Error())
	}
	receipt := transactionToReceipt(msg, wallet)
	return c.JSON(map[string]interface{}{
		"receipt": receipt,
	})
}

func (app *ApiServer) handleRelay(ctx context.Context, decodedTx *v1.ManageEntityLegacy) (*v1.Transaction, error) {
	// submit tx to core
	res, err := app.openAudioSDK.Core.SendTransaction(ctx, connect.NewRequest(&v1.SendTransactionRequest{
		Transaction: &v1.SignedTransaction{
			Transaction: &v1.SignedTransaction_ManageEntity{
				ManageEntity: decodedTx,
			},
		},
	}))
	if err != nil {
		return nil, err
	}

	msg := res.Msg.Transaction
	return msg, nil
}

func transactionToReceipt(tx *v1.Transaction, wallet string) map[string]interface{} {
	return map[string]interface{}{
		"transactionHash":   tx.GetHash(),
		"blockHash":         tx.GetBlockHash(),
		"blockNumber":       tx.GetHeight(),
		"transactionIndex":  0,
		"from":              wallet,
		"to":                wallet,
		"gasUsed":           10,
		"cumulativeGasUsed": 10,
		"effectiveGasPrice": 420,
		"status":            true,
	}
}
