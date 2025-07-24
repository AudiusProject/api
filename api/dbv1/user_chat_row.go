package dbv1

import (
	"database/sql"
	"encoding/json"
	"time"

	"bridgerton.audius.co/trashid"
)

type UserChatRow struct {
	ChatID                 string            `db:"chat_id" json:"chat_id"`
	CreatedAt              time.Time         `db:"created_at" json:"created_at"`
	LastMessage            *string           `db:"last_message" json:"last_message"`
	LastMessageAt          *time.Time        `db:"last_message_at" json:"last_message_at"`
	LastMessageIsPlaintext bool              `db:"last_message_is_plaintext" json:"last_message_is_plaintext"`
	InviteCode             string            `db:"invite_code" json:"invite_code"`
	LastActiveAt           sql.NullTime      `db:"last_active_at" json:"last_read_at"`
	UnreadCount            int32             `db:"unread_count" json:"unread_message_count"`
	ClearedHistoryAt       sql.NullTime      `db:"cleared_history_at" json:"cleared_history_at"`
	IsBlast                bool              `db:"is_blast" json:"is_blast"`
	Audience               sql.NullString    `db:"audience" json:"audience"`
	AudienceContentType    *string           `db:"audience_content_type" json:"audience_content_type,omitempty"`
	AudienceContentID      *trashid.HashId   `db:"audience_content_id" json:"audience_content_id,omitempty"`
	ChatMembers            []UserChatMembers `db:"members" json:"chat_members"`
}

type UserChatMembers struct {
	UserID           trashid.HashId `db:"user_id" json:"user_id"`
	ClearedHistoryAt sql.NullTime   `db:"cleared_history_at" json:"-"`
}

func (row UserChatRow) MarshalJSON() ([]byte, error) {
	type Alias UserChatRow

	audience := ""
	if row.Audience.Valid {
		audience = row.Audience.String
	}

	lastActiveAt := ""
	if row.LastActiveAt.Valid {
		lastActiveAt = row.LastActiveAt.Time.UTC().Format(time.RFC3339Nano)
	}

	clearedHistoryAt := ""
	if row.ClearedHistoryAt.Valid {
		clearedHistoryAt = row.ClearedHistoryAt.Time.UTC().Format(time.RFC3339Nano)
	}

	recheckPermissions := false
	for _, member := range row.ChatMembers {
		if member.ClearedHistoryAt.Valid && member.ClearedHistoryAt.Time.After(row.LastMessageAt) {
			recheckPermissions = true
		}
	}

	return json.Marshal(&struct {
		Alias
		Audience           string `json:"audience"`
		ClearedHistoryAt   string `json:"cleared_history_at"`
		RecheckPermissions bool   `json:"recheck_permissions"`
		LastActiveAt       string `json:"last_read_at"`
	}{
		Alias:              Alias(row),
		Audience:           audience,
		ClearedHistoryAt:   clearedHistoryAt,
		RecheckPermissions: recheckPermissions,
		LastActiveAt:       lastActiveAt,
	})
}
