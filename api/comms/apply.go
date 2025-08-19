package comms

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"

	// "comms.audius.co/discovery/config"
	// "comms.audius.co/discovery/db"
	// "comms.audius.co/discovery/db/queries"
	// "comms.audius.co/discovery/misc"
	// "comms.audius.co/discovery/schema"
	// "github.com/jmoiron/sqlx"

	"github.com/tidwall/gjson"
	// "gorm.io/gorm/logger"
)

type RPCProcessor struct {
	sync.Mutex
	pool      *dbv1.DBPools
	validator *Validator

	// TODO
	discoveryConfig *config.DiscoveryConfig
}

func NewProcessor(pool *dbv1.DBPools, discoveryConfig *config.DiscoveryConfig) (*RPCProcessor, error) {

	// set up validator + limiter
	limiter, err := NewRateLimiter()
	if err != nil {
		return nil, err
	}

	aaoServer := "https://discoveryprovider.audius.co"
	if discoveryConfig.IsStaging {
		aaoServer = "https://discoveryprovider.staging.audius.co"
	}

	if discoveryConfig.IsDev {
		aaoServer = "http://audius-protocol-discovery-provider-1"
	}

	validator := &Validator{
		pool:      pool,
		limiter:   limiter,
		aaoServer: aaoServer,
	}

	proc := &RPCProcessor{
		validator:       validator,
		discoveryConfig: discoveryConfig,
	}

	return proc, nil
}

// TODO: replace logger
// Clean up wallet recovery (do we even need it?)
// Change format or at least naming of RpcLog
// Do we still need to check for already applied?
// - Maybe the validation needs to happen inside a transaction, since we check for existing stuff there?

// Validates + applies a message
func (proc *RPCProcessor) Apply(rpcLog *RpcLog) error {

	logger := slog.With("sig", rpcLog.Sig)
	var err error

	// check for already applied
	var exists int
	db.Conn.Get(&exists, `select count(*) from rpc_log where sig = $1`, rpcLog.Sig)
	if exists == 1 {
		logger.Debug("rpc already in log, skipping duplicate", "sig", rpcLog.Sig)
		return nil
	}

	startTime := time.Now()
	takeSplit := func() time.Duration {
		split := time.Since(startTime)
		startTime = time.Now()
		return split
	}

	// validate signing wallet
	wallet, err := misc.RecoverWallet(rpcLog.Rpc, rpcLog.Sig)
	if err != nil {
		logger.Warn("unable to recover wallet, skipping")
		return nil
	}
	logger.Debug("recovered wallet", "took", takeSplit())

	if wallet != rpcLog.FromWallet {
		logger.Warn("recovered wallet no match", "recovered", wallet, "expected", rpcLog.FromWallet, "realeyd_by", rpcLog.RelayedBy)
		return nil
	}

	// parse raw rpc
	var rawRpc RawRPC
	err = json.Unmarshal(rpcLog.Rpc, &rawRpc)
	if err != nil {
		logger.Info(err.Error())
		return nil
	}

	// check for "internal" message...
	if strings.HasPrefix(rawRpc.Method, "internal.") {
		err := proc.applyInternalMessage(rpcLog, &rawRpc)
		if err != nil {
			logger.Info("failed to apply internal rpc", "error", err)
		} else {
			logger.Info("applied internal RPC", "sig", rpcLog.Sig)
		}
		return nil
	}

	// get ts
	messageTs := rpcLog.RelayedAt

	userId, err := GetRPCCurrentUserID(rpcLog, &rawRpc)
	if err != nil {
		logger.Info("unable to get user ID")
		return err // or nil?
	}

	// for debugging
	chatId := gjson.GetBytes(rpcLog.Rpc, "params.chat_id").String()

	logger = logger.With(
		"wallet", wallet,
		"userId", userId,
		"relayed_by", rpcLog.RelayedBy,
		"relayed_at", rpcLog.RelayedAt,
		"chat_id", chatId,
		"sig", rpcLog.Sig)
	logger.Debug("got user", "took", takeSplit())

	attemptApply := func() error {

		// write to db
		tx, err := db.Conn.Beginx()
		if err != nil {
			return err
		}
		defer tx.Rollback()

		logger.Debug("begin tx", "took", takeSplit(), "sig", rpcLog.Sig)

		switch RPCMethod(rawRpc.Method) {
		case RPCMethodChatCreate:
			var params ChatCreateRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}
			err = chatCreate(tx, userId, messageTs, params)
			if err != nil {
				return err
			}
		case RPCMethodChatDelete:
			var params ChatDeleteRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}
			err = chatDelete(tx, userId, params.ChatID, messageTs)
			if err != nil {
				return err
			}
		case RPCMethodChatMessage:
			var params ChatMessageRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}
			err = chatSendMessage(tx, userId, params.ChatID, params.MessageID, messageTs, params.Message)
			if err != nil {
				return err
			}
		case RPCMethodChatReact:
			var params ChatReactRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}
			err = chatReactMessage(tx, userId, params.ChatID, params.MessageID, params.Reaction, messageTs)
			if err != nil {
				return err
			}

		case RPCMethodChatRead:
			var params ChatReadRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}
			// do nothing if last active at >= message timestamp
			lastActive, err := queries.LastActiveAt(tx, context.Background(), queries.ChatMembershipParams{
				ChatID: params.ChatID,
				UserID: userId,
			})
			if err != nil {
				return err
			}
			if !lastActive.Valid || messageTs.After(lastActive.Time) {
				err = chatReadMessages(tx, userId, params.ChatID, messageTs)
				if err != nil {
					return err
				}
			}
		case RPCMethodChatPermit:
			var params ChatPermitRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}
			err = chatSetPermissions(tx, userId, params.Permit, params.PermitList, params.Allow, messageTs)
			if err != nil {
				return err
			}
		case RPCMethodChatBlock:
			var params ChatBlockRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}
			blockeeUserId, err := trashid.DecodeHashId(params.UserID)
			if err != nil {
				return err
			}
			err = chatBlock(tx, userId, int32(blockeeUserId), messageTs)
			if err != nil {
				return err
			}
		case RPCMethodChatUnblock:
			var params ChatUnblockRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}
			unblockedUserId, err := trashid.DecodeHashId(params.UserID)
			if err != nil {
				return err
			}
			err = chatUnblock(tx, userId, int32(unblockedUserId), messageTs)
			if err != nil {
				return err
			}

		case RPCMethodChatBlast:
			var params ChatBlastRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}

			outgoingMessages, err := chatBlast(tx, userId, messageTs, params)
			if err != nil {
				return err
			}
			// Send chat message websocket event to all recipients who have existing chats
			for _, outgoingMessage := range outgoingMessages {
				j, err := json.Marshal(outgoingMessage.ChatMessageRPC)
				if err != nil {
					slog.Error("err: invalid json", "err", err)
				} else {
					// TODO
					// websocketNotify(json.RawMessage(j), userId, messageTs.Round(time.Microsecond))
				}
			}
		default:
			logger.Warn("no handler for ", rawRpc.Method)
		}

		logger.Debug("called handler", "took", takeSplit())

		err = tx.Commit()
		if err != nil {
			return err
		}
		logger.Debug("commited", "took", takeSplit())

		// TODO
		// send out websocket events fire + forget style
		// websocketNotify(rpcLog.Rpc, userId, messageTs.Round(time.Microsecond))
		logger.Debug("websocket push done", "took", takeSplit())

		return nil
	}

	err = attemptApply()
	if err != nil {
		logger.Warn("apply failed", "err", err)
	}
	return err
}

// func websocketNotify(rpcJson json.RawMessage, userId int32, timestamp time.Time) {
// 	if chatId := gjson.GetBytes(rpcJson, "params.chat_id").String(); chatId != "" {

// 		var userIds []int32
// 		err := db.Conn.Select(&userIds, `select user_id from chat_member where chat_id = $1 and is_hidden = false`, chatId)
// 		if err != nil {
// 			logger.Warn("failed to load chat members for websocket push " + err.Error())
// 			return
// 		}

// 		for _, receiverUserId := range userIds {
// 			websocketPush(userId, receiverUserId, rpcJson, timestamp)
// 		}
// 	} else if gjson.GetBytes(rpcJson, "method").String() == "chat.blast" {
// 		go func() {
// 			// Add delay before broadcasting blast messages - see PAY-3573
// 			time.Sleep(30 * time.Second)
// 			websocketPushAll(userId, rpcJson, timestamp)
// 		}()
// 	}
// }
