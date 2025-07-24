package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetChatsParams struct {
	CurrentUserID string     `query:"current_user_id"`
	Before        *time.Time `query:"before"`
	After         *time.Time `query:"after"`
	Limit         int        `query:"limit" default:"50" validate:"min=1,max=100"`
}

func (app *ApiServer) getChats(c *fiber.Ctx) error {
	sql := `
	-- Get User Chats
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
		false AS is_blast,
		'' AS audience,
		NULL AS audience_content_type,
		NULL AS audience_content_id,
		(
			SELECT json_agg(json_build_object(
				'user_id', chat_member.user_id,
				'cleared_history_at', chat_member.cleared_history_at
			))
			FROM chat_member
			WHERE chat_member.chat_id = chat.chat_id
		)::jsonb AS members
	FROM chat_member
	JOIN chat ON chat.chat_id = chat_member.chat_id
	WHERE chat_member.user_id = @user_id
	AND chat_member.is_hidden = false
	AND chat.last_message IS NOT NULL
		AND chat.last_message_at < @before
		AND chat.last_message_at > @after
	AND (chat_member.cleared_history_at IS NULL
		OR chat.last_message_at > chat_member.cleared_history_at)

	-- Add blasts as well
	UNION ALL (
		SELECT DISTINCT ON (audience, audience_content_type, audience_content_id)
			concat_ws(':', audience, audience_content_type,
					CASE
						WHEN audience_content_id IS NOT NULL THEN id_encode(audience_content_id)
						ELSE NULL
					END) AS chat_id,
			MIN(created_at) OVER (PARTITION BY audience, audience_content_type, audience_content_id) AS created_at,
			plaintext AS last_message,
				MAX(created_at) OVER (PARTITION BY audience, audience_content_type, audience_content_id) AS last_message_at,
			TRUE AS last_message_is_plaintext,
			'' AS invite_code,
			created_at AS last_active_at,
			0 AS unread_count,
			NULL AS cleared_history_at,
			TRUE AS is_blast,
			audience,
			audience_content_type,
			audience_content_id,
			json_build_array()::jsonb AS members
		FROM chat_blast
		WHERE from_user_id = @user_id
			AND created_at < @before
			AND created_at > @after
		ORDER BY
			audience,
			audience_content_type,
			audience_content_id,
			created_at DESC
	)

	ORDER BY last_message_at DESC, is_blast DESC, chat_id ASC
	LIMIT @limit
	;
	`

	sqlSummary := `
	-- User Chats Summary
	WITH user_chats AS (
		SELECT
			chat.chat_id,
			chat.last_message_at
		FROM chat_member
		JOIN chat ON chat.chat_id = chat_member.chat_id
		WHERE chat_member.user_id = @user_id
			AND chat.last_message IS NOT NULL
			AND (chat_member.cleared_history_at IS NULL
				OR chat.last_message_at > chat_member.cleared_history_at)
	)
	SELECT
		(SELECT COUNT(*) AS total_count FROM user_chats) AS total_count,
		(
			SELECT COUNT(*) FROM user_chats
			WHERE last_message_at < @before
		) AS before_count,
		(
			SELECT COUNT(*) FROM user_chats
			WHERE last_message_at > @after
		) AS after_count,
		@before AS prev,
		@after AS next
	;
	`

	params := &GetChatsParams{}

	err := app.ParseAndValidateQueryParams(c, params)
	if err != nil {
		return err
	}

	userId := 0
	if params.CurrentUserID != "" {
		userId, err = trashid.DecodeHashId(params.CurrentUserID)
	} else {
		wallet := app.getAuthedWallet(c)
		userId, err = app.getUserIDFromWallet(c.Context(), wallet)
	}
	if err != nil {
		return err
	}

	beforeCursorPos := time.Now().UTC()
	afterCursorPos := time.Time{}
	if params.Before != nil {
		beforeCursorPos = *params.Before
	}
	if params.After != nil {
		afterCursorPos = *params.After
	}

	rawRows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id": userId,
		"before":  beforeCursorPos,
		"after":   afterCursorPos,
		"limit":   params.Limit,
	})
	if err != nil {
		return err
	}

	rows, err := pgx.CollectRows(rawRows, pgx.RowToStructByName[dbv1.UserChatRow])
	if err != nil {
		return err
	}

	if len(rows) > 0 {
		lastRow := rows[len(rows)-1]
		if lastRow.LastMessageAt != nil {
			beforeCursorPos = *lastRow.LastMessageAt
		} else {
			beforeCursorPos = lastRow.CreatedAt
		}
		firstRow := rows[0]
		if firstRow.LastMessageAt != nil {
			afterCursorPos = *firstRow.LastMessageAt
		} else {
			afterCursorPos = firstRow.CreatedAt
		}
	}
	summaryRaw, err := app.pool.Query(c.Context(), sqlSummary, pgx.NamedArgs{
		"user_id": userId,
		"before":  beforeCursorPos,
		"after":   afterCursorPos,
	})
	if err != nil {
		return err
	}
	summary, err := pgx.CollectExactlyOneRow(summaryRaw, pgx.RowToStructByName[CommsSummary])
	if err != nil {
		return err
	}

	return c.JSON(CommsResponse{
		Data:    rows,
		Summary: &summary,
		Health: CommsHealth{
			IsHealthy: true,
		},
	})
}
