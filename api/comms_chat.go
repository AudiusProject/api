package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetChatRouteParams struct {
	ChatID string `params:"chatId"`
}

func (api *ApiServer) getChat(c *fiber.Ctx) error {
	sql := `
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
	FALSE AS is_blast,
	NULL AS audience,
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
	WHERE chat_member.user_id = @user_id AND chat_member.chat_id = @chat_id

	-- Query blasts as well
	UNION ALL (
		SELECT DISTINCT ON (audience, audience_content_type, audience_content_id)
			concat_ws(':', audience, audience_content_type, 
					CASE 
						WHEN audience_content_id IS NOT NULL THEN id_encode(audience_content_id)
						ELSE NULL 
					END) AS chat_id,
			min(created_at) OVER (PARTITION BY audience, audience_content_type, audience_content_id) AS created_at,
			plaintext AS last_message,
				max(created_at) over (PARTITION BY audience, audience_content_type, audience_content_id) AS last_message_at,
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
		FROM chat_blast b
		WHERE from_user_id = @user_id
			AND concat_ws(':', audience, audience_content_type, 
					CASE 
						WHEN audience_content_id IS NOT NULL THEN id_encode(audience_content_id)
						ELSE NULL 
					END) = @chat_id
		ORDER BY
			audience,
			audience_content_type,
			audience_content_id,
			created_at DESC
	)
	`

	params := &GetChatRouteParams{}
	err := c.ParamsParser(params)
	if err != nil {
		return err
	}

	wallet := api.getAuthedWallet(c)
	userId, err := api.getUserIDFromWallet(c.Context(), wallet)
	if err != nil {
		return err
	}

	rawRows, err := api.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id": userId,
		"chat_id": params.ChatID,
	})
	if err != nil {
		return err
	}

	row, err := pgx.CollectExactlyOneRow(rawRows, pgx.RowToStructByName[dbv1.UserChatRow])
	if err != nil {
		return err
	}

	return c.JSON(CommsResponse{
		Data: row,
		Health: CommsHealth{
			IsHealthy: true,
		},
	})
}
