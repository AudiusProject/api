package dbv1

import (
	"encoding/json"
	"errors"
	"time"

	"bridgerton.audius.co/trashid"
)

type ChatMessageAndReactionsRow struct {
	MessageID   string                   `db:"message_id" json:"message_id"`
	ChatID      string                   `db:"chat_id" json:"-"`
	UserID      trashid.HashId           `db:"user_id" json:"sender_user_id"`
	CreatedAt   time.Time                `db:"created_at" json:"created_at"`
	Audience    string                   `db:"audience" json:"audience"`
	Ciphertext  string                   `db:"ciphertext" json:"message"`
	IsPlaintext bool                     `db:"is_plaintext" json:"is_plaintext"`
	Reactions   []ChatMessageReactionRow `json:"reactions"`
}

type ChatMessageReactionRow struct {
	UserID    trashid.HashId `db:"user_id" json:"user_id"`
	MessageID string         `db:"message_id" json:"-"`
	Reaction  string         `db:"reaction" json:"reaction"`
	CreatedAt JSONTime       `db:"created_at" json:"created_at"`
	UpdatedAt JSONTime       `db:"updated_at" json:"-"`
}

type JSONTime struct {
	time.Time
}

type Reactions []ChatMessageReactionRow

func (reactions *Reactions) Scan(value interface{}) error {
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
