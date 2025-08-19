package comms

import (
	"context"
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5"
)

/*
todo:

- maybe blast_id should be computed like: `md5(from_user_id || audience || plaintext)`

*/
// Result struct to hold chat_id and to_user_id
type ChatBlastResult struct {
	ChatID   string `db:"chat_id"`
	ToUserID int32  `db:"to_user_id"`
}

type OutgoingChatMessage struct {
	ChatMessageRPC ChatMessageRPC `json:"chat_message_rpc"`
}

func chatBlast(tx pgx.Tx, ctx context.Context, userId int32, ts time.Time, params ChatBlastRPCParams) ([]OutgoingChatMessage, error) {
	var audienceContentID *int
	if params.AudienceContentID != nil {
		id, _ := trashid.DecodeHashId(*params.AudienceContentID)
		audienceContentID = &id
	}

	// insert params.Message into chat_blast table
	_, err := tx.Exec(ctx, `
		insert into chat_blast
			(blast_id, from_user_id, audience, audience_content_type, audience_content_id, plaintext, created_at)
		values
			($1, $2, $3, $4, $5, $6, $7)
		on conflict (blast_id)
		do nothing
		`, params.BlastID, userId, params.Audience, params.AudienceContentType, audienceContentID, params.Message, ts)
	if err != nil {
		return nil, err
	}

	// fan out messages to existing threads
	// see also: similar but subtly different inverse query in `getNewBlasts helper in chat.go`
	var results []ChatBlastResult

	fanOutSql := `
	WITH targ AS (
		SELECT
			blast_id,
			from_user_id,
			to_user_id,
			member_b.chat_id
		FROM chat_blast
		JOIN chat_blast_audience(chat_blast.blast_id) USING (blast_id)
		LEFT JOIN chat_member member_a on from_user_id = member_a.user_id
		LEFT JOIN chat_member member_b on to_user_id = member_b.user_id and member_b.chat_id = member_a.chat_id
		WHERE blast_id = $1
		AND member_b.chat_id IS NOT NULL
		AND chat_allowed(from_user_id, to_user_id)
	),
	insert_message AS (
		INSERT INTO chat_message
			(message_id, chat_id, user_id, created_at, blast_id)
		SELECT
			blast_id || targ.chat_id, -- this ordering needs to match Misc.BlastMessageID
			targ.chat_id,
			targ.from_user_id,
			$2,
			blast_id
		FROM targ
		ON conflict do nothing
	)
	SELECT chat_id FROM targ;
	`

	rows, err := tx.Query(ctx, fanOutSql, params.BlastID, ts)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Scan the results into the results slice
	results, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (ChatBlastResult, error) {
		var result ChatBlastResult
		err := row.Scan(&result.ChatID, &result.ToUserID)
		return result, err
	})
	if err != nil {
		return nil, err
	}

	// Formulate chat rpc messages for recipients who have an existing chat with sender
	var outgoingMessages []OutgoingChatMessage
	for _, result := range results {
		messageID := BlastMessageID(params.BlastID, result.ChatID)

		isPlaintext := true
		outgoingMessages = append(outgoingMessages, OutgoingChatMessage{
			ChatMessageRPC: ChatMessageRPC{
				Method: MethodChatMessage,
				Params: ChatMessageRPCParams{
					ChatID:      result.ChatID,
					Message:     params.Message,
					MessageID:   messageID,
					IsPlaintext: &isPlaintext,
					Audience:    &params.Audience,
				}}})

		if err := chatUpdateLatestFields(tx, ctx, result.ChatID); err != nil {
			return nil, err
		}
	}

	return outgoingMessages, nil
}
