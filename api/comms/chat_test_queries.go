package comms

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

/**
The queries in this file were migrated from protocol to support the existing tests without
needing extensive modifications. They are not meant to be used in endpoints or productions logic
**/

type chatMessagesAndReactionsParams struct {
	UserID  int32     `db:"user_id" json:"user_id"`
	ChatID  string    `db:"chat_id" json:"chat_id"`
	Limit   int32     `json:"limit"`
	Before  time.Time `json:"before"`
	After   time.Time `json:"after"`
	IsBlast bool      `json:"is_blast"`
}
type chatMessageAndReactionsRow struct {
	MessageID   string    `db:"message_id" json:"message_id"`
	ChatID      string    `db:"chat_id" json:"chat_id"`
	UserID      int32     `db:"user_id" json:"user_id"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	Ciphertext  string    `db:"ciphertext" json:"ciphertext"`
	IsPlaintext bool      `db:"is_plaintext" json:"is_plaintext"`
	Reactions   reactions `json:"reactions"`
}
type chatMessageReactionRow struct {
	UserID    int32    `db:"user_id" json:"user_id"`
	MessageID string   `db:"message_id" json:"message_id"`
	Reaction  string   `db:"reaction" json:"reaction"`
	CreatedAt JSONTime `db:"created_at" json:"created_at"`
	UpdatedAt JSONTime `db:"updated_at" json:"updated_at"`
}

type JSONTime struct {
	time.Time
}

// Override JSONB timestamp unmarshaling since the postgres driver
// does not convert timestamp strings in JSON -> time.Time
func (t *JSONTime) UnmarshalJSON(b []byte) error {
	timeformat := "2006-01-02T15:04:05.999999"
	var timestamp string
	err := json.Unmarshal(b, &timestamp)
	if err != nil {
		return err
	}
	t.Time, err = time.Parse(timeformat, timestamp)
	if err != nil {
		return err
	}
	return nil
}

type reactions []chatMessageReactionRow

func (reactions *reactions) Scan(value interface{}) error {
	if value == nil {
		*reactions = nil
		return nil
	}

	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, reactions)
	case string:
		return json.Unmarshal([]byte(v), reactions)
	default:
		return errors.New("type assertion failed: expected []byte or string for JSON scanning")
	}
}

func getChatMessagesAndReactions(db dbv1.DBTX, ctx context.Context, arg chatMessagesAndReactionsParams) ([]chatMessageAndReactionsRow, error) {
	// special case to handle outgoing blasts...
	if arg.IsBlast {
		parts := strings.Split(arg.ChatID, ":")
		if len(parts) < 1 {
			return nil, errors.New("bad request: invalid blast id")
		}
		audience := parts[0]

		if ChatBlastAudience(audience) == FollowerAudience ||
			ChatBlastAudience(audience) == TipperAudience ||
			ChatBlastAudience(audience) == CustomerAudience ||
			ChatBlastAudience(audience) == RemixerAudience {

			result, err := db.Query(ctx, `
			SELECT
				b.blast_id as message_id,
				$2 as chat_id,
				b.from_user_id as user_id,
				b.created_at,
				b.plaintext as ciphertext,
				true as is_plaintext,
				'[]'::json AS reactions
			FROM chat_blast b
			WHERE b.from_user_id = $1
				AND concat_ws(':', audience, audience_content_type,
					CASE
						WHEN audience_content_id IS NOT NULL THEN id_encode(audience_content_id)
						ELSE NULL
					END) = $2
			  AND b.created_at < $3
			  AND b.created_at > $4
			ORDER BY b.created_at DESC
			LIMIT $5
			`,
				arg.UserID,
				arg.ChatID,
				arg.Before,
				arg.After,
				arg.Limit,
			)
			if err != nil {
				return nil, err
			}

			return pgx.CollectRows(result, pgx.RowToStructByName[chatMessageAndReactionsRow])
		} else {
			return nil, errors.New("bad request: unsupported audience " + audience)
		}
	}

	result, err := db.Query(ctx, `
		SELECT
			chat_message.message_id,
			chat_message.chat_id,
			chat_message.user_id,
			chat_message.created_at,
			COALESCE(chat_message.ciphertext, chat_blast.plaintext) as ciphertext,
			chat_blast.plaintext is not null as is_plaintext,
			to_json(array(select row_to_json(r) from chat_message_reactions r where chat_message.message_id = r.message_id)) AS reactions
		FROM chat_message
		JOIN chat_member ON chat_message.chat_id = chat_member.chat_id
		LEFT JOIN chat_blast USING (blast_id)
		WHERE chat_member.user_id = $1
			AND chat_message.chat_id = $2
			AND chat_message.created_at < $4
			AND chat_message.created_at > $5
			AND (chat_member.cleared_history_at IS NULL
				OR chat_message.created_at > chat_member.cleared_history_at
			)
		ORDER BY chat_message.created_at DESC, chat_message.message_id
		LIMIT $3`,
		arg.UserID,
		arg.ChatID,
		arg.Limit,
		arg.Before,
		arg.After,
	)
	if err != nil {
		return nil, err
	}

	return pgx.CollectRows(result, pgx.RowToStructByName[chatMessageAndReactionsRow])
}

type chatMembershipParams struct {
	UserID int32  `db:"user_id" json:"user_id"`
	ChatID string `db:"chat_id" json:"chat_id"`
}

type userChatRow struct {
	ChatID                 string           `db:"chat_id" json:"chat_id"`
	CreatedAt              time.Time        `db:"created_at" json:"created_at"`
	LastMessage            pgtype.Text      `db:"last_message" json:"last_message"`
	LastMessageAt          time.Time        `db:"last_message_at" json:"last_message_at"`
	LastMessageIsPlaintext bool             `db:"last_message_is_plaintext" json:"last_message_is_plaintext"`
	InviteCode             string           `db:"invite_code" json:"invite_code"`
	LastActiveAt           pgtype.Timestamp `db:"last_active_at" json:"last_active_at"`
	UnreadCount            int32            `db:"unread_count" json:"unread_count"`
	ClearedHistoryAt       pgtype.Timestamp `db:"cleared_history_at" json:"cleared_history_at"`
	IsBlast                bool             `db:"is_blast" json:"is_blast"`
	Audience               pgtype.Text      `db:"audience" json:"audience"`
	AudienceContentType    pgtype.Text      `db:"audience_content_type" json:"audience_content_type"`
	AudienceContentID      pgtype.Int4      `db:"audience_content_id" json:"audience_content_id"`
}

const userChatQuery = `
SELECT
  chat.chat_id,
  chat.created_at,
  chat.last_message,
  chat.last_message_at,
  chat.last_message_is_plaintext,
  chat_member.invite_code,
  chat_member.last_active_at,
  chat_member.unread_count,
  chat_member.cleared_history_at,
	false as is_blast,
	null as audience,
	null as audience_content_type,
	null as audience_content_id
FROM chat_member
JOIN chat ON chat.chat_id = chat_member.chat_id
WHERE chat_member.user_id = $1 AND chat_member.chat_id = $2

union all (

  SELECT DISTINCT ON (audience, audience_content_type, audience_content_id)
    concat_ws(':', audience, audience_content_type,
			CASE
				WHEN audience_content_id IS NOT NULL THEN id_encode(audience_content_id)
				ELSE NULL
			END) as chat_id,
    min(created_at) over (partition by audience, audience_content_type, audience_content_id) as created_at,
    plaintext as last_message,
		max(created_at) over (partition by audience, audience_content_type, audience_content_id) as last_message_at,
    true as last_message_is_plaintext,
    '' as invite_code,
    created_at as last_active_at,
    0 as unread_count,
    null as cleared_history_at,
		true as is_blast,
		audience,
		audience_content_type,
		audience_content_id
  FROM chat_blast b
  WHERE from_user_id = $1
    AND concat_ws(':', audience, audience_content_type,
			CASE
				WHEN audience_content_id IS NOT NULL THEN id_encode(audience_content_id)
				ELSE NULL
			END) = $2
  ORDER BY
    audience,
    audience_content_type,
    audience_content_id,
    created_at DESC
)
`

func getUserChat(db dbv1.DBTX, ctx context.Context, arg chatMembershipParams) (userChatRow, error) {
	rows, err := db.Query(ctx, userChatQuery,
		arg.UserID,
		arg.ChatID,
	)
	if err != nil {
		return userChatRow{}, err
	}
	defer rows.Close()

	row, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[userChatRow])
	if err != nil {
		return userChatRow{}, err
	}
	return row, nil
}

type userChatsParams struct {
	UserID int32     `db:"user_id" json:"user_id"`
	Limit  int32     `json:"limit"`
	Before time.Time `json:"before"`
	After  time.Time `json:"after"`
}

const userChatsQuery = `
SELECT
  chat.chat_id,
  chat.created_at,
  chat.last_message,
  chat.last_message_at,
  chat.last_message_is_plaintext,
  chat_member.invite_code,
  chat_member.last_active_at,
  chat_member.unread_count,
  chat_member.cleared_history_at,
	false as is_blast,
	null as audience,
	null as audience_content_type,
	null as audience_content_id
FROM chat_member
JOIN chat ON chat.chat_id = chat_member.chat_id
WHERE chat_member.user_id = $1
  AND chat_member.is_hidden = false
  AND chat.last_message IS NOT NULL
	AND chat.last_message_at < $3
	AND chat.last_message_at > $4
  AND (chat_member.cleared_history_at IS NULL
	  OR chat.last_message_at > chat_member.cleared_history_at)


union all (

  SELECT DISTINCT ON (audience, audience_content_type, audience_content_id)
    concat_ws(':', audience, audience_content_type,
			CASE
				WHEN audience_content_id IS NOT NULL THEN id_encode(audience_content_id)
				ELSE NULL
			END) as chat_id,
    min(created_at) over (partition by audience, audience_content_type, audience_content_id) as created_at,
    plaintext as last_message,
		max(created_at) over (partition by audience, audience_content_type, audience_content_id) as last_message_at,
    true as last_message_is_plaintext,
    '' as invite_code,
    created_at as last_active_at,
    0 as unread_count,
    null as cleared_history_at,
		true as is_blast,
		audience,
		audience_content_type,
		audience_content_id
  FROM chat_blast b
  WHERE from_user_id = $1
	AND b.created_at < $3
	AND b.created_at > $4
  ORDER BY
    audience,
    audience_content_type,
    audience_content_id,
    created_at DESC
)

ORDER BY last_message_at DESC, is_blast DESC, chat_id ASC
LIMIT $2
`

func getUserChats(db dbv1.DBTX, ctx context.Context, arg userChatsParams) ([]userChatRow, error) {
	rows, err := db.Query(ctx, userChatsQuery, arg.UserID, arg.Limit, arg.Before, arg.After)
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[userChatRow])
}
