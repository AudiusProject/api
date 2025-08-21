package comms

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

var (
	ErrMessageRateLimitExceeded = errors.New("user has exceeded the maximum number of new messages")
)

type Validator struct {
	logger    *zap.Logger
	pool      *dbv1.DBPools
	limiter   *RateLimiter
	aaoServer string
}

func NewValidator(pool *dbv1.DBPools, limiter *RateLimiter, config *config.Config, logger *zap.Logger) *Validator {
	return &Validator{
		pool:      pool,
		limiter:   limiter,
		aaoServer: config.AAOServer,
		logger:    logger,
	}
}

func (vtor *Validator) Validate(ctx context.Context, userId int32, rawRpc RawRPC) error {
	methodName := RPCMethod(rawRpc.Method)

	// banned?
	isBanned, err := vtor.isBanned(ctx, userId)
	if err != nil {
		return err
	}
	if isBanned {
		return fmt.Errorf("user_id %d is banned from chat", userId)
	}

	switch methodName {
	case RPCMethodChatCreate:
		return vtor.validateChatCreate(ctx, userId, rawRpc)
	case RPCMethodChatDelete:
		return vtor.validateChatDelete(userId, rawRpc)
	case RPCMethodChatMessage:
		return vtor.validateChatMessage(ctx, userId, rawRpc)
	case RPCMethodChatReact:
		return vtor.validateChatReact(vtor.pool, ctx, userId, rawRpc)
	case RPCMethodChatRead:
		return vtor.validateChatRead(userId, rawRpc)
	case RPCMethodChatPermit:
		return vtor.validateChatPermit(userId, rawRpc)
	case RPCMethodChatBlock:
		return vtor.validateChatBlock(userId, rawRpc)
	case RPCMethodChatUnblock:
		return vtor.validateChatUnblock(userId, rawRpc)
	default:
		vtor.logger.Debug("no validator for " + rawRpc.Method)
	}

	return nil
}

func (vtor *Validator) isBanned(ctx context.Context, userId int32) (bool, error) {
	isBanned := false
	err := vtor.pool.QueryRow(ctx, "select count(user_id) = 1 from chat_ban where user_id = $1 and is_banned = true", userId).Scan(&isBanned)
	if err != nil {
		return false, err
	}
	return isBanned, nil
}

func (vtor *Validator) validateChatCreate(ctx context.Context, userId int32, rpc RawRPC) error {
	// validate rpc.params valid
	var params ChatCreateRPCParams
	err := json.Unmarshal(rpc.Params, &params)
	if err != nil {
		return err
	}

	// validate chatId does not already exist
	query := "select count(*) from chat where chat_id = $1;"
	var chatCount int
	err = vtor.pool.QueryRow(ctx, query, params.ChatID).Scan(&chatCount)
	if err != nil {
		return err
	}
	if chatCount != 0 {
		return errors.New("Chat already exists")
	}

	if len(params.Invites) != 2 {
		return errors.New("Chat must have 2 members")
	}

	user1, err := trashid.DecodeHashId(params.Invites[0].UserID)
	if err != nil {
		return err
	}
	user2, err := trashid.DecodeHashId(params.Invites[1].UserID)
	if err != nil {
		return err
	}

	receiver := int32(user1)
	if receiver == userId {
		receiver = int32(user2)
	}

	// Check that the creator is non-abusive
	err = validateSenderPassesAbuseCheck(vtor.pool, ctx, vtor.logger, userId, vtor.aaoServer)
	if err != nil {
		return err
	}

	// if recipient is creating a chat from a blast
	// we ignore the receiver's inbox settings
	// because receiver has sent a blast to this user.
	{
		hasBlast, err := hasNewBlastFromUser(vtor.pool, ctx, userId, receiver)
		if err != nil {
			return err
		}
		if hasBlast {
			return nil
		}
	}

	// validate receiver permits chats from sender
	err = validatePermissions(vtor.pool, ctx, userId, receiver)
	if err != nil {
		return err
	}

	// validate does not exceed new chat rate limit for any invited users
	var users []int32
	for _, invite := range params.Invites {
		userId, err := trashid.DecodeHashId(invite.UserID)
		if err != nil {
			return err
		}
		users = append(users, int32(userId))
	}
	err = vtor.validateNewChatRateLimit(vtor.pool, ctx, users)
	if err != nil {
		return err
	}

	return nil
}

func (vtor *Validator) validateChatMessage(ctx context.Context, userId int32, rpc RawRPC) error {
	// validate rpc.params valid
	var params ChatMessageRPCParams
	err := json.Unmarshal(rpc.Params, &params)
	if err != nil {
		return err
	}

	// validate userId is a member of chatId in good standing
	err = validateChatMembership(vtor.pool, ctx, userId, params.ChatID)
	if err != nil {
		return err
	}

	// validate not blocked and can chat according to receiver's inbox permission settings
	err = validatePermittedToMessage(vtor.pool, ctx, userId, params.ChatID)
	if err != nil {
		return err
	}

	// validate does not exceed new message rate limit
	err = vtor.validateNewMessageRateLimit(vtor.pool, ctx, userId, params.ChatID)
	if err != nil {
		return err
	}

	return nil
}

func (vtor *Validator) validateChatReact(pool *dbv1.DBPools, ctx context.Context, userId int32, rpc RawRPC) error {
	// validate rpc.params valid
	var params ChatReactRPCParams
	err := json.Unmarshal(rpc.Params, &params)
	if err != nil {
		return err
	}

	// validate userId is a member of chatId in good standing
	err = validateChatMembership(vtor.pool, ctx, userId, params.ChatID)
	if err != nil {
		return err
	}

	// validate message exists in chat
	var exists bool
	err = pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM chat_message WHERE chat_id = $1 AND message_id = $2)`, params.ChatID, params.MessageID).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("message does not exist in chat")
	}

	// validate not blocked and can chat according to receiver's inbox permission settings
	err = validatePermittedToMessage(vtor.pool, ctx, userId, params.ChatID)
	if err != nil {
		return err
	}

	return nil
}

func (vtor *Validator) validateChatRead(userId int32, rpc RawRPC) error {
	// validate rpc.params valid
	var params ChatReadRPCParams
	err := json.Unmarshal(rpc.Params, &params)
	if err != nil {
		return err
	}

	// validate userId is a member of chatId in good standing
	err = validateChatMembership(vtor.pool, context.Background(), userId, params.ChatID)
	if err != nil {
		return err
	}

	return nil
}

func (vtor *Validator) validateChatPermit(userId int32, rpc RawRPC) error {
	// validate rpc.params valid
	var params ChatPermitRPCParams
	err := json.Unmarshal(rpc.Params, &params)
	if err != nil {
		return err
	}

	return nil
}

func (vtor *Validator) validateChatBlock(userId int32, rpc RawRPC) error {
	// validate rpc.params valid
	var params ChatBlockRPCParams
	err := json.Unmarshal(rpc.Params, &params)
	if err != nil {
		return err
	}

	return nil
}

func (vtor *Validator) validateChatUnblock(userId int32, rpc RawRPC) error {
	// validate rpc.params valid
	var params ChatBlockRPCParams
	err := json.Unmarshal(rpc.Params, &params)
	if err != nil {
		return err
	}

	// validate that params.UserID is currently blocked by userId
	blockeeUserId, err := trashid.DecodeHashId(params.UserID)
	if err != nil {
		return err
	}

	var exists bool
	err = vtor.pool.QueryRow(context.Background(), `
		select exists(
			select 1 from chat_blocked_users
			where blocker_user_id = $1 and blockee_user_id = $2
		)
	`, userId, blockeeUserId).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("user is not blocked")
	}

	return nil
}

func (vtor *Validator) validateChatDelete(userId int32, rpc RawRPC) error {
	// validate rpc.params valid
	var params ChatDeleteRPCParams
	err := json.Unmarshal(rpc.Params, &params)
	if err != nil {
		return err
	}

	// validate userId is a member of chatId in good standing
	err = validateChatMembership(vtor.pool, context.Background(), userId, params.ChatID)
	if err != nil {
		return err
	}

	return nil
}

// Calculate cursor from rate limit timeframe
func (vtor *Validator) calculateRateLimitCursor(timeframe int) time.Time {
	return time.Now().UTC().Add(-time.Hour * time.Duration(timeframe))
}

func (vtor *Validator) validateNewChatRateLimit(pool *dbv1.DBPools, ctx context.Context, users []int32) error {
	var err error

	// rate_limit_seconds

	limiter := vtor.limiter
	timeframe := limiter.Get(RateLimitTimeframeHours)

	// Max num of new chats permitted per timeframe
	maxNumChats := limiter.Get(RateLimitMaxNumNewChats)

	cursor := vtor.calculateRateLimitCursor(timeframe)

	// Build the query with proper placeholders for the IN clause
	query := `
	WITH counts AS (
		SELECT COUNT(*) AS count
		FROM chat
		JOIN chat_member on chat.chat_id = chat_member.chat_id
		WHERE chat_member.user_id = ANY($1) AND chat.created_at > $2
		GROUP BY chat_member.user_id
	)
	SELECT COALESCE(MAX(count), 0) FROM counts;
	`

	var numChats int
	err = pool.QueryRow(ctx, query, users, cursor).Scan(&numChats)
	if err != nil {
		return err
	}
	if numChats >= maxNumChats {
		vtor.logger.Info("hit rate limit (new chats)", zap.Any("users", users))
		return errors.New("An invited user has exceeded the maximum number of new chats")
	}

	return nil
}

func (vtor *Validator) validateNewMessageRateLimit(pool *dbv1.DBPools, ctx context.Context, userId int32, chatId string) error {
	var err error

	// BurstRateLimit
	{
		query := `
		select
			coalesce(sum(case when created_at > now() - interval '1 second' then 1 else 0 end), 0) as s1,
			coalesce(sum(case when created_at > now() - interval '10 seconds' then 1 else 0 end), 0) as s10,
			coalesce(sum(case when created_at > now() - interval '60 seconds' then 1 else 0 end), 0) as s60
		from chat_message
		where user_id = $1
		and created_at > now() - interval '60 seconds';
		`
		var s1, s10, s60 int64
		err = pool.QueryRow(ctx, query, userId).Scan(&s1, &s10, &s60)
		if err != nil {
			vtor.logger.Error("burst rate limit query failed", zap.Error(err))
		}

		// 10 per second in last second
		if s1 > 10 {
			vtor.logger.Warn("message rate limit exceeded", zap.String("bucket", "1s"), zap.Int32("user_id", userId), zap.Int64("count", s1))
			return ErrMessageRateLimitExceeded

		}

		// 7 per second for last 10 seconds
		if s10 > 70 {
			vtor.logger.Warn("message rate limit exceeded", zap.String("bucket", "10s"), zap.Int32("user_id", userId), zap.Int64("count", s10))
			return ErrMessageRateLimitExceeded
		}

		// 5 per second for last 60 seconds
		if s60 > 300 {
			vtor.logger.Warn("message rate limit exceeded", zap.String("bucket", "60s"), zap.Int32("user_id", userId), zap.Int64("count", s60))
			return ErrMessageRateLimitExceeded
		}
	}

	limiter := vtor.limiter
	timeframe := limiter.Get(RateLimitTimeframeHours)

	// Max number of new messages permitted per timeframe
	maxNumMessages := limiter.Get(RateLimitMaxNumMessages)

	// Max number of new messages permitted per recipient (chat) per timeframe
	maxNumMessagesPerRecipient := limiter.Get(RateLimitMaxNumMessagesPerRecipient)

	// Cursor for rate limit timeframe
	cursor := vtor.calculateRateLimitCursor(timeframe)

	// Check total message count and max messages per chat
	query := `
	WITH counts_per_chat AS (
		SELECT COUNT(*)
		FROM chat_message
		WHERE user_id = $1 and created_at > $2
		GROUP BY chat_id
	)
	SELECT COALESCE(SUM(count), 0) AS total_count, COALESCE(MAX(count), 0) as max_count_per_chat FROM counts_per_chat;
	`

	var totalCount, maxCountPerChat int
	err = pool.QueryRow(ctx, query, userId, cursor).Scan(&totalCount, &maxCountPerChat)
	if err != nil {
		return err
	}
	if totalCount >= maxNumMessages || maxCountPerChat >= maxNumMessagesPerRecipient {
		if totalCount >= maxNumMessages {
			vtor.logger.Info("hit rate limit (total count new messages)", zap.Int32("user", userId), zap.String("chat", chatId))
		}
		if maxCountPerChat >= maxNumMessagesPerRecipient {
			vtor.logger.Info("hit rate limit (new messages per recipient)", zap.Int32("user", userId), zap.String("chat", chatId))
		}
		return ErrMessageRateLimitExceeded
	}

	return nil
}

func validateChatMembership(pool *dbv1.DBPools, ctx context.Context, userId int32, chatId string) error {
	var exists bool
	err := pool.QueryRow(ctx, `select exists(select 1 from chat_member where user_id = $1 and chat_id = $2)`, userId, chatId).Scan(&exists)
	if err != nil {
		return err
	}
	if !exists {
		return errors.New("user is not a member of this chat")
	}
	return nil
}

func validatePermissions(pool *dbv1.DBPools, ctx context.Context, sender int32, receiver int32) error {
	permissionFailure := errors.New("Not permitted to send messages to this user")

	ok := false
	err := pool.QueryRow(ctx, `select chat_allowed($1, $2)`, sender, receiver).Scan(&ok)
	if err != nil {
		return err
	}
	if !ok {
		return permissionFailure
	}
	return nil

}

func validatePermittedToMessage(pool *dbv1.DBPools, ctx context.Context, userId int32, chatId string) error {
	// Single query that validates:
	// 1. Chat has exactly 2 members
	// 2. User is a member of the chat
	// 3. User has permission to message the other member
	query := `
		WITH chat_members AS (
			SELECT user_id
			FROM chat_member
			WHERE chat_id = $1
		),
		member_count AS (
			SELECT COUNT(*) as count
			FROM chat_members
		),
		other_member AS (
			SELECT user_id
			FROM chat_members
			WHERE user_id != $2
		)
		SELECT
			CASE
				WHEN mc.count != 2 THEN false
				WHEN NOT EXISTS (SELECT 1 FROM chat_members WHERE user_id = $2) THEN false
				WHEN NOT chat_allowed($2, om.user_id) THEN false
				ELSE true
			END as is_permitted
		FROM member_count mc
		CROSS JOIN other_member om
	`

	var isPermitted bool
	err := pool.QueryRow(ctx, query, chatId, userId).Scan(&isPermitted)
	if err != nil {
		if err == pgx.ErrNoRows {
			return errors.New("Chat must have 2 members")
		}
		return err
	}

	if !isPermitted {
		return errors.New("Not permitted to send messages to this user")
	}

	return nil
}

var ErrAttestationFailed = errors.New("attestation failed")

// TODO: Better AAO usage that corresponds to the claim rewards code
func validateSenderPassesAbuseCheck(pool *dbv1.DBPools, ctx context.Context, logger *zap.Logger, userId int32, aaoServer string) error {
	// Keeping this somewhat opaque as it gets sent to client
	var handle string
	err := pool.QueryRow(ctx, `SELECT handle FROM users WHERE user_id = $1`, userId).Scan(&handle)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fmt.Errorf("user %d not found", userId)
		}
		return err
	}

	url := fmt.Sprintf("%s/attestation/%s", aaoServer, handle)
	// Dummy challenge for now to mitigate
	requestBody := []byte(`{ "challengeId": "x", "challengeSpecifier": "x", "amount": 0 }`)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(requestBody))
	if err != nil {
		logger.Error("Error checking user attestation", zap.Error(err), zap.String("handle", handle))
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		logger.Warn("User failed AAO check", zap.Int32("userId", userId), zap.Int("status", resp.StatusCode), zap.String("aaoServer", aaoServer))
		return ErrAttestationFailed
	}
	return nil
}

// hasNewBlastFromUser efficiently checks if a new blast exists from a specific user
// without fetching all blast data. Returns true if a valid blast exists, false otherwise.
func hasNewBlastFromUser(pool *dbv1.DBPools, ctx context.Context, userID int32, fromUserID int32) (bool, error) {
	// Construct the expected chat ID for this user pair
	expectedChatID := trashid.ChatID(int(userID), int(fromUserID))

	// This query checks for the existence of a new blast from a specific user
	// using the same logic as GetNewBlasts but optimized for existence check
	var hasNewBlast = `
	with
	last_permission_change as (
		select max(t) as t from (
			select updated_at as t from chat_permissions where user_id = $1
			union
			select created_at as t from chat_blocked_users where blocker_user_id = $1
			union
			select to_timestamp(0)
		) as timestamp_subquery
	)
	select exists(
		select 1
		from chat_blast blast
		where
		blast.from_user_id = $2
		and blast.created_at > (select t from last_permission_change)
		and chat_allowed(blast.from_user_id, $1)
		and not exists (
			select 1 from chat_member cm
			where cm.user_id = $1 and cm.chat_id = $3
		)
		and (
			-- follower_audience
			(blast.audience = 'follower_audience' and exists (
				SELECT 1
				FROM follows
				WHERE follows.followee_user_id = blast.from_user_id
					AND follows.follower_user_id = $1
					AND follows.is_delete = false
					AND follows.created_at < blast.created_at
			))
			OR
			-- tipper_audience
			(blast.audience = 'tipper_audience' and exists (
				SELECT 1
				FROM user_tips tip
				WHERE receiver_user_id = blast.from_user_id
				AND sender_user_id = $1
				AND tip.created_at < blast.created_at
			))
			OR
			-- remixer_audience
			(blast.audience = 'remixer_audience' and exists (
				SELECT 1
				FROM tracks t
				JOIN remixes ON remixes.child_track_id = t.track_id
				JOIN tracks og ON remixes.parent_track_id = og.track_id
				WHERE og.owner_id = blast.from_user_id
					AND t.owner_id = $1
					AND (
						blast.audience_content_id IS NULL
						OR (
							blast.audience_content_type = 'track'
							AND blast.audience_content_id = og.track_id
						)
					)
			))
			OR
			-- customer_audience
			(blast.audience = 'customer_audience' and exists (
				SELECT 1
				FROM usdc_purchases p
				WHERE p.seller_user_id = blast.from_user_id
					AND p.buyer_user_id = $1
					AND (
						blast.audience_content_id IS NULL
						OR (
							blast.audience_content_type = p.content_type::text
							AND blast.audience_content_id = p.content_id
						)
					)
			))
			OR
			-- coin_holder_audience
			(blast.audience = 'coin_holder_audience' and exists (
				SELECT 1
				FROM artist_coins ac
				JOIN sol_user_balances sub ON sub.mint = ac.mint
				WHERE ac.user_id = blast.from_user_id
					AND sub.user_id = $1
					AND sub.balance > 0
					-- TODO: PE-6663 This isn't entirely correct yet, need to check "time of most recent membership"
					AND sub.created_at < blast.created_at
			))
		)
	)`

	var exists bool
	err := pool.QueryRow(ctx, hasNewBlast, userID, fromUserID, expectedChatID).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}
