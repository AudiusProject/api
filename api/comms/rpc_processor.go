package comms

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/trashid"
	"go.uber.org/zap"

	"github.com/gofiber/contrib/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgxlisten"
	"github.com/tidwall/gjson"
)

var rpcLogChannel = "rpc_log_inserted"

type rpcLogInsertedNotification struct {
	Signature string `json:"sig"`
}

type RPCProcessor struct {
	sync.Mutex
	pool             *dbv1.DBPools
	writePool        *pgxpool.Pool
	validator        *Validator
	logger           *zap.Logger
	websocketManager *CommsWebsocketManager

	// PostgreSQL LISTEN/NOTIFY fields
	listener     *pgxlisten.Listener
	listenCtx    context.Context
	listenCancel context.CancelFunc
	listenWg     sync.WaitGroup
}

func NewProcessor(pool *dbv1.DBPools, writePool *pgxpool.Pool, config *config.Config, logger *zap.Logger) (*RPCProcessor, error) {
	ctx, cancel := context.WithCancel(context.Background())
	// set up validator
	validator := NewValidator(pool, DefaultRateLimitConfig, config, logger)
	websocketManager := NewCommsWebsocketManager(logger)

	proc := &RPCProcessor{
		validator:        validator,
		pool:             pool,
		writePool:        writePool,
		logger:           logger,
		websocketManager: websocketManager,

		listenCtx:    ctx,
		listenCancel: cancel,
	}

	return proc, nil
}

func (proc *RPCProcessor) Validate(ctx context.Context, userId int32, rawRpc RawRPC) error {
	return proc.validator.Validate(ctx, userId, rawRpc)
}

// Validates + applies a message
func (proc *RPCProcessor) Apply(ctx context.Context, rpcLog *RpcLog) error {
	logger := proc.logger.With(
		zap.String("sig", rpcLog.Sig),
	)
	var err error

	var exists int
	proc.pool.QueryRow(ctx, `select count(*) from rpc_log where sig = $1`, rpcLog.Sig).Scan(&exists)
	if exists == 1 {
		logger.Debug("rpc already in log, skipping duplicate", zap.String("sig", rpcLog.Sig))
		return nil
	}

	startTime := time.Now()
	takeSplit := func() time.Duration {
		split := time.Since(startTime)
		startTime = time.Now()
		return split
	}

	// validate signing wallet
	wallet, err := recoverSigningWallet(rpcLog.Sig, rpcLog.Rpc)
	if err != nil {
		logger.Warn("unable to recover wallet, skipping")
		return nil
	}
	logger.Debug("recovered wallet", zap.Duration("took", takeSplit()))

	if wallet != rpcLog.FromWallet {
		logger.Warn("recovered wallet no match", zap.String("recovered", wallet), zap.String("expected", rpcLog.FromWallet), zap.String("realeyd_by", rpcLog.RelayedBy))
		return nil
	}

	// parse raw rpc
	var rawRpc RawRPC
	err = json.Unmarshal(rpcLog.Rpc, &rawRpc)
	if err != nil {
		logger.Info(err.Error())
		return nil
	}

	// check for "internal" message, which are from the legacy implementation
	if strings.HasPrefix(rawRpc.Method, "internal.") {
		logger.Warn("recieved internal message, skipping")
		return nil
	}

	// get ts
	messageTs := rpcLog.RelayedAt

	userId, err := proc.GetRPCCurrentUserID(ctx, rpcLog, &rawRpc)
	if err != nil {
		logger.Info("unable to get user ID")
		return err // or nil?
	}

	// for debugging
	chatId := gjson.GetBytes(rpcLog.Rpc, "params.chat_id").String()

	logger = logger.With(
		zap.String("wallet", wallet),
		zap.Int32("userId", userId),
		zap.String("relayed_by", rpcLog.RelayedBy),
		zap.Time("relayed_at", rpcLog.RelayedAt),
		zap.String("chat_id", chatId),
		zap.String("sig", rpcLog.Sig),
	)
	logger.Debug("got user", zap.Duration("took", takeSplit()))

	attemptApply := func() error {

		// write to db
		tx, err := proc.writePool.Begin(ctx)
		if err != nil {
			return err
		}
		defer tx.Rollback(ctx)

		logger.Debug("begin tx", zap.Duration("took", takeSplit()), zap.String("sig", rpcLog.Sig))

		count, err := insertRpcLogRow(tx, ctx, rpcLog)
		if err != nil {
			return err
		}
		if count == 0 {
			// No rows were inserted because the sig (id) is already in rpc_log.
			// Do not process redelivered messages that have already been processed.
			logger.Info("rpc already in log, skipping duplicate", zap.String("sig", rpcLog.Sig))
			return nil
		}
		logger.Debug("inserted RPC", zap.Duration("took", takeSplit()))

		switch RPCMethod(rawRpc.Method) {
		case RPCMethodChatCreate:
			var params ChatCreateRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}
			err = chatCreate(tx, ctx, userId, messageTs, params)
			if err != nil {
				return err
			}
		case RPCMethodChatDelete:
			var params ChatDeleteRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}
			err = chatDelete(tx, ctx, userId, params.ChatID, messageTs)
			if err != nil {
				return err
			}
		case RPCMethodChatMessage:
			var params ChatMessageRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}
			err = chatSendMessage(tx, ctx, userId, params.ChatID, params.MessageID, messageTs, params.Message)
			if err != nil {
				return err
			}
		case RPCMethodChatReact:
			var params ChatReactRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}
			err = chatReactMessage(tx, ctx, userId, params.ChatID, params.MessageID, params.Reaction, messageTs)
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
			var lastActive pgtype.Timestamp
			const lastActiveAtQuery = `
select last_active_at from chat_member where chat_id = $1 and user_id = $2`

			err = tx.QueryRow(ctx, lastActiveAtQuery, params.ChatID, userId).Scan(&lastActive)
			if err != nil {
				return err
			}

			if !lastActive.Valid || messageTs.After(lastActive.Time) {
				err = chatReadMessages(tx, ctx, userId, params.ChatID, messageTs)
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
			err = chatSetPermissions(tx, ctx, userId, params.Permit, params.PermitList, params.Allow, messageTs)
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
			err = chatBlock(tx, ctx, userId, int32(blockeeUserId), messageTs)
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
			err = chatUnblock(tx, ctx, userId, int32(unblockedUserId), messageTs)
			if err != nil {
				return err
			}

		case RPCMethodChatBlast:
			var params ChatBlastRPCParams
			err = json.Unmarshal(rawRpc.Params, &params)
			if err != nil {
				return err
			}

			outgoingMessages, err := chatBlast(tx, ctx, userId, messageTs, params)
			if err != nil {
				return err
			}
			// Send chat message websocket event to all recipients who have existing chats
			for _, outgoingMessage := range outgoingMessages {
				_, err := json.Marshal(outgoingMessage.ChatMessageRPC)
				if err != nil {
					logger.Error("err: invalid json", zap.Error(err))
				} else {
					// TODO
					// websocketNotify(json.RawMessage(j), userId, messageTs.Round(time.Microsecond))
				}
			}
		default:
			logger.Warn("no handler for ", zap.String("method", rawRpc.Method))
		}

		logger.Debug("called handler", zap.Duration("took", takeSplit()))

		err = tx.Commit(ctx)
		if err != nil {
			return err
		}
		logger.Debug("commited", zap.Duration("took", takeSplit()))

		// TODO
		// send out websocket events fire + forget style
		// websocketNotify(rpcLog.Rpc, userId, messageTs.Round(time.Microsecond))
		logger.Debug("websocket push done", zap.Duration("took", takeSplit()))

		return nil
	}

	err = attemptApply()
	if err != nil {
		logger.Warn("apply failed", zap.Error(err))
	}
	return err
}

func (proc *RPCProcessor) GetRPCCurrentUserID(ctx context.Context, rpcLog *RpcLog, rawRpc *RawRPC) (int32, error) {
	walletAddress := rpcLog.FromWallet
	encodedCurrentUserId := rawRpc.CurrentUserID

	// attempt to read the (newly added) current_user_id field
	if encodedCurrentUserId != "" {
		if u, err := trashid.DecodeHashId(encodedCurrentUserId); err == nil && u > 0 {

			const checkCurrentUserQuery = `
			select count(*) > 0
			from users
			where is_current = true
				and user_id = $1
				and wallet = lower($2)
				and handle IS NOT NULL
				and is_available = TRUE
				and is_deactivated = FALSE
			`
			// valid current_user_id + wallet combo?
			// for now just check that the pair exists in the user table
			// in the future this can check a "grants" table that a given operation is permitted
			isValid := false
			err := proc.pool.QueryRow(ctx, checkCurrentUserQuery, u, walletAddress).Scan(&isValid)
			if err == nil && isValid {
				return int32(u), nil
			} else {
				proc.logger.Warn("invalid current_user_id", zap.Error(err), zap.String("wallet", walletAddress), zap.String("encoded_user_id", encodedCurrentUserId), zap.Int("user_id", u))
			}
		}
	}

	const getUserIDFromWalletQuery = `
		select user_id
		from users
		where is_current = TRUE
		and handle IS NOT NULL
		and is_available = TRUE
		and is_deactivated = FALSE
		and wallet = LOWER($1)
		order by user_id asc
		`
	// fallback to looking up user_id using wallet alone
	var userId int32
	err := proc.pool.QueryRow(ctx, getUserIDFromWalletQuery, walletAddress).Scan(&userId)
	return userId, err
}

func insertRpcLogRow(db dbv1.DBTX, ctx context.Context, rpcLog *RpcLog) (int64, error) {
	query := `
		INSERT INTO rpc_log (relayed_by, relayed_at, applied_at, from_wallet, rpc, sig)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT DO NOTHING
		`
	result, err := db.Exec(ctx, query, rpcLog.RelayedBy, rpcLog.RelayedAt, time.Now(), rpcLog.FromWallet, rpcLog.Rpc, rpcLog.Sig)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

/** Watch for pg_notify() on new rpc_logs so we can send websocket events to the appropriate users */
func (proc *RPCProcessor) StartListening() error {
	if proc.listener != nil {
		return nil // Already listening
	}

	proc.listener = &pgxlisten.Listener{
		Connect: func(ctx context.Context) (*pgx.Conn, error) {
			// Use the write pool for listening to ensure we get notifications from the primary database
			conn, err := proc.writePool.Acquire(ctx)
			if err != nil {
				return nil, err
			}
			return conn.Conn(), nil
		},
		LogError: func(ctx context.Context, err error) {
			proc.logger.Error("Comms RPC pg_notify listener error", zap.Error(err))
		},
		ReconnectDelay: 10 * time.Second,
	}

	proc.listener.Handle(rpcLogChannel, pgxlisten.HandlerFunc(proc.handleRpcLogInserted))

	// Start listening in a goroutine
	proc.listenWg.Add(1)
	go func() {
		defer proc.listenWg.Done()
		if err := proc.listener.Listen(proc.listenCtx); err != nil {
			proc.logger.Error("Comms RPC pg_notify listener failed", zap.Error(err))
		}
	}()

	proc.logger.Info("Started listening for comms rpc_log insertions")
	return nil
}

// StopListening stops the PostgreSQL listener
func (proc *RPCProcessor) StopListening() {
	if proc.listenCancel != nil {
		proc.listenCancel()
	}

	if proc.listener != nil {
		// The listener will be stopped when the context is cancelled
		proc.listener = nil
	}

	// Wait for the listener goroutine to finish
	proc.listenWg.Wait()
	proc.logger.Info("Stopped listening for comms rpc_log insertions")
}

// handleRpcLogInserted processes incoming PostgreSQL notifications
func (proc *RPCProcessor) handleRpcLogInserted(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
	proc.logger.Debug("Received PostgreSQL notification",
		zap.String("channel", notification.Channel),
		zap.String("payload", notification.Payload))

	// Parse the notification payload
	var payload rpcLogInsertedNotification
	if err := json.Unmarshal([]byte(notification.Payload), &payload); err != nil {
		proc.logger.Error("Failed to parse notification payload", zap.Error(err))
		return err
	}

	var rpcLog RpcLog
	err := proc.writePool.QueryRow(ctx, `select rpc from rpc_log where sig = $1`, payload.Signature).Scan(&rpcLog)
	if err != nil {
		proc.logger.Error("Failed to query rpc log", zap.Error(err))
		return err
	}

	// parse raw rpc
	var rawRpc RawRPC
	err = json.Unmarshal(rpcLog.Rpc, &rawRpc)
	if err != nil {
		proc.logger.Error("failed to parse raw rpc " + err.Error())
		return err
	}

	senderUserId, err := proc.GetRPCCurrentUserID(ctx, &rpcLog, &rawRpc)
	if err != nil {
		proc.logger.Error("failed to get current user id for websocket push " + err.Error())
		return err
	}

	if chatId := gjson.GetBytes(rpcLog.Rpc, "params.chat_id").String(); chatId != "" {
		userRows, err := proc.writePool.Query(ctx, `select user_id from chat_member where chat_id = $1 and is_hidden = false`, chatId)
		if err != nil {
			proc.logger.Error("failed to load chat members for websocket push " + err.Error())
			// TODO: Will this mess anything up?
			return err
		}
		userIds, err := pgx.CollectRows(userRows, pgx.RowTo[int32])
		if err != nil {
			proc.logger.Error("failed to collect user ids for websocket push " + err.Error())
			// TODO: Will this mess anything up?
			return err
		}

		for _, receiverUserId := range userIds {
			proc.websocketManager.WebsocketPush(int32(senderUserId), receiverUserId, rpcLog.Rpc, rpcLog.AppliedAt)
		}
	} else if gjson.GetBytes(rpcLog.Rpc, "method").String() == "chat.blast" {
		go func() {
			// Add delay before broadcasting blast messages - see PAY-3573
			time.Sleep(30 * time.Second)
			proc.websocketManager.WebsocketPushAll(int32(senderUserId), rpcLog.Rpc, rpcLog.AppliedAt)
		}()
	}
	return nil
}

func (proc *RPCProcessor) RegisterWebsocket(userId int32, conn *websocket.Conn) {
	proc.websocketManager.RegisterWebsocket(userId, conn)
}
