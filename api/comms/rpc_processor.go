package comms

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/trashid"
	"go.uber.org/zap"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/contrib/websocket"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgxlisten"
	"github.com/tidwall/gjson"
)

var (
	chatMessageInsertedChannel = "chat_message_inserted"
	chatBlastInsertedChannel   = "chat_blast_inserted"
	chatMessageReactionChanged = "chat_message_reaction_changed"
)

type chatMessageInsertedNotification struct {
	MessageID string `json:"message_id"`
}

type chatBlastInsertedNotification struct {
	BlastID string `json:"blast_id"`
}

type chatMessageReactionInsertedNotification struct {
	MessageID string  `json:"message_id"`
	UserID    int32   `json:"user_id"`
	Reaction  *string `json:"reaction"`
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
	wallet, _, err := RecoverSigningWallet(rpcLog.Sig, rpcLog.Rpc)
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
	messageTs := rpcLog.RelayedAt.UTC()

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

			_, err = chatBlast(tx, ctx, userId, messageTs, params)
			if err != nil {
				return err
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
	result, err := db.Exec(ctx, query, rpcLog.RelayedBy, rpcLog.RelayedAt.UTC(), time.Now().UTC(), rpcLog.FromWallet, rpcLog.Rpc, rpcLog.Sig)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}

/** Watch for pg_notify() on new chat messages and blast messages so we can send websocket events to the appropriate users */
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

	proc.listener.Handle(chatMessageInsertedChannel, pgxlisten.HandlerFunc(proc.handleChatMessageInserted))
	proc.listener.Handle(chatBlastInsertedChannel, pgxlisten.HandlerFunc(proc.handleChatBlastInserted))
	proc.listener.Handle(chatMessageReactionChanged, pgxlisten.HandlerFunc(proc.handleChatMessageReactionChanged))

	// Start listening in a goroutine
	proc.listenWg.Add(1)
	go func() {
		defer proc.listenWg.Done()
		if err := proc.listener.Listen(proc.listenCtx); err != nil {
			proc.logger.Error("Comms RPC pg_notify listener failed", zap.Error(err))
		}
	}()

	proc.logger.Info("Started listening for comms chat_message_inserted, chat_blast_inserted, and chat_message_reaction_inserted notifications")
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
	proc.logger.Info("Stopped listening for comms chat_message_inserted, chat_blast_inserted, and chat_message_reaction_inserted notifications")
}

func (proc *RPCProcessor) handleChatMessageInserted(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
	proc.logger.Debug("Received PostgreSQL notification for chat message",
		zap.String("channel", notification.Channel),
		zap.String("payload", notification.Payload))

	var payload chatMessageInsertedNotification
	if err := json.Unmarshal([]byte(notification.Payload), &payload); err != nil {
		proc.logger.Error("Failed to parse chat message notification payload", zap.Error(err))
		return err
	}

	type InsertedChatMessage struct {
		MessageID   string      `db:"message_id"`
		ChatID      string      `db:"chat_id"`
		UserID      int32       `db:"user_id"`
		CreatedAt   time.Time   `db:"created_at"`
		Ciphertext  pgtype.Text `db:"ciphertext"`
		IsPlaintext bool        `db:"is_plaintext"`
	}
	// Joins on blasts to get message text if the origin was a blast
	row, err := proc.writePool.Query(ctx, `
		SELECT
			chat_message.message_id,
			chat_message.chat_id,
			chat_message.user_id,
			chat_message.created_at,
			COALESCE(chat_message.ciphertext, chat_blast.plaintext) AS ciphertext,
			chat_blast.plaintext IS NOT NULL as is_plaintext
		FROM chat_message
		JOIN chat_member ON chat_message.chat_id = chat_member.chat_id
		LEFT JOIN chat_blast USING (blast_id)
		WHERE message_id = $1`, payload.MessageID)
	if err != nil {
		proc.logger.Error("Failed to query chat message", zap.Error(err))
		return err
	}
	chatMessage, err := pgx.CollectOneRow(row, pgx.RowToStructByName[InsertedChatMessage])
	if err != nil {
		proc.logger.Error("Failed to collect chat message", zap.Error(err))
		return err
	}

	// Get chat members to notify
	userRows, err := proc.writePool.Query(ctx, `select user_id from chat_member where chat_id = $1 and is_hidden = false`, chatMessage.ChatID)
	if err != nil {
		proc.logger.Error("failed to load chat members for websocket push " + err.Error())
		return err
	}
	userIds, err := pgx.CollectRows(userRows, pgx.RowTo[int32])
	if err != nil {
		proc.logger.Error("failed to collect user ids for websocket push " + err.Error())
		return err
	}

	messageData := ChatMessageRPC{
		Method: MethodChatMessage,
		Params: ChatMessageRPCParams{
			ChatID:      chatMessage.ChatID,
			MessageID:   chatMessage.MessageID,
			IsPlaintext: &chatMessage.IsPlaintext,
			Message:     chatMessage.Ciphertext.String,
		},
	}

	messageJson, err := json.Marshal(messageData)
	if err != nil {
		proc.logger.Error("Failed to marshal message data", zap.Error(err))
		return err
	}

	messageTs := chatMessage.CreatedAt.UTC().Round(time.Microsecond)

	// Send to all chat members except the sender
	for _, receiverUserId := range userIds {
		if receiverUserId != chatMessage.UserID {
			proc.websocketManager.WebsocketPush(chatMessage.UserID, receiverUserId, messageJson, messageTs)
		}
	}

	return nil
}

func (proc *RPCProcessor) handleChatBlastInserted(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
	proc.logger.Debug("Received PostgreSQL notification for chat blast",
		zap.String("channel", notification.Channel),
		zap.String("payload", notification.Payload))

	var payload chatBlastInsertedNotification
	if err := json.Unmarshal([]byte(notification.Payload), &payload); err != nil {
		proc.logger.Error("Failed to parse chat blast notification payload", zap.Error(err))
		return err
	}

	row, err := proc.writePool.Query(ctx, `
		SELECT blast_id, from_user_id, audience, audience_content_id, plaintext, created_at, audience_content_type
		FROM chat_blast
		WHERE blast_id = $1`, payload.BlastID)
	if err != nil {
		proc.logger.Error("Failed to query chat blast", zap.Error(err))
		return err
	}
	chatBlast, err := pgx.CollectOneRow(row, pgx.RowToStructByName[dbv1.ChatBlast])
	if err != nil {
		proc.logger.Error("Failed to collect chat blast", zap.Error(err))
		return err
	}

	blastData := ChatBlastRPC{
		Method: MethodChatBlast,
		Params: ChatBlastRPCParams{
			BlastID:  chatBlast.BlastID,
			Audience: ChatBlastAudience(chatBlast.Audience),
			Message:  chatBlast.Plaintext,
		},
	}

	if chatBlast.AudienceContentID.Valid {
		audienceContentID, err := trashid.EncodeHashId(int(chatBlast.AudienceContentID.Int32))
		if err != nil {
			proc.logger.Error("Failed to encode audience content id", zap.Error(err))
			return err
		}
		blastData.Params.AudienceContentID = &audienceContentID
	}
	if chatBlast.AudienceContentType.Valid {
		audienceContentType := AudienceContentType(chatBlast.AudienceContentType.String)
		blastData.Params.AudienceContentType = &audienceContentType
	}

	blastJson, err := json.Marshal(blastData)
	if err != nil {
		proc.logger.Error("Failed to marshal blast data", zap.Error(err))
		return err
	}

	blastTs := chatBlast.CreatedAt.Time.Round(time.Microsecond)

	// Send to all connected clients (blast messages go to everyone)
	go func() {
		proc.websocketManager.WebsocketPushAll(chatBlast.FromUserID, blastJson, blastTs)
	}()

	return nil
}

func (proc *RPCProcessor) handleChatMessageReactionChanged(ctx context.Context, notification *pgconn.Notification, conn *pgx.Conn) error {
	proc.logger.Debug("Received PostgreSQL notification for chat message reaction",
		zap.String("channel", notification.Channel),
		zap.String("payload", notification.Payload))

	// Parse the notification payload
	var payload chatMessageReactionInsertedNotification
	if err := json.Unmarshal([]byte(notification.Payload), &payload); err != nil {
		proc.logger.Error("Failed to parse chat message reaction notification payload", zap.Error(err))
		return err
	}

	// Get the chat_id for this message to find members to notify
	var chatID string
	err := proc.writePool.QueryRow(ctx, `SELECT chat_id FROM chat_message WHERE message_id = $1`, payload.MessageID).Scan(&chatID)
	if err != nil {
		proc.logger.Error("Failed to get chat_id for message", zap.Error(err))
		return err
	}

	// Get chat members to notify
	userRows, err := proc.writePool.Query(ctx, `select user_id from chat_member where chat_id = $1 and is_hidden = false`, chatID)
	if err != nil {
		proc.logger.Error("failed to load chat members for websocket push " + err.Error())
		return err
	}
	userIds, err := pgx.CollectRows(userRows, pgx.RowTo[int32])
	if err != nil {
		proc.logger.Error("failed to collect user ids for websocket push " + err.Error())
		return err
	}

	reactionData := ChatReactRPC{
		Method: MethodChatReact,
		Params: ChatReactRPCParams{
			ChatID:    chatID,
			MessageID: payload.MessageID,
			Reaction:  payload.Reaction,
		},
	}

	reactionJson, err := json.Marshal(reactionData)
	if err != nil {
		proc.logger.Error("Failed to marshal reaction data", zap.Error(err))
		return err
	}

	// Use current time since we don't have the timestamp in the notification
	reactionTs := time.Now().UTC().Round(time.Microsecond)

	// Send to all chat members except the sender
	for _, receiverUserId := range userIds {
		if receiverUserId != payload.UserID {
			proc.websocketManager.WebsocketPush(payload.UserID, receiverUserId, reactionJson, reactionTs)
		}
	}

	return nil
}

func (proc *RPCProcessor) RegisterWebsocket(userId int32, conn *websocket.Conn) {
	proc.websocketManager.RegisterWebsocket(userId, conn)
}

func (proc *RPCProcessor) SetPubkeyForUser(userId int32, pubkey *ecdsa.PublicKey) {
	pubkeyBytes := crypto.FromECDSAPub(pubkey)
	pubkeyBase64 := base64.StdEncoding.EncodeToString(pubkeyBytes)
	_, err := proc.writePool.Exec(context.Background(), `insert into user_pubkeys values ($1, $2) on conflict do nothing`, userId, pubkeyBase64)
	if err != nil {
		proc.logger.Warn("failed to set pubkey for user", zap.Error(err))
	}
}
