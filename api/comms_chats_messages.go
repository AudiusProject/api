package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetChatMessageRouteParams struct {
	ChatID string `params:"chatId"`
}

type GetChatMessagesQueryParams struct {
	IsBlast bool       `query:"is_blast"`
	Before  *time.Time `query:"before"`
	After   *time.Time `query:"after"`
	Limit   int        `query:"limit"`
}

func (api ApiServer) getChatMessages(c *fiber.Ctx) error {
	sql := `
	SELECT
		chat_message.message_id,
		chat_message.chat_id,
		chat_message.user_id,
		chat_message.created_at,
		COALESCE(chat_message.ciphertext, chat_blast.plaintext) AS ciphertext,
		chat_blast.plaintext IS NOT NULL as is_plaintext,
		to_json(array(SELECT row_to_json(r) FROM chat_message_reactions r WHERE chat_message.message_id = r.message_id)) AS reactions
	FROM chat_message
	JOIN chat_member ON chat_message.chat_id = chat_member.chat_id
	LEFT JOIN chat_blast USING (blast_id)
	WHERE chat_member.user_id = @user_id
		AND chat_message.chat_id = @chat_id
		AND chat_message.created_at < @before
		AND chat_message.created_at > @after
		AND (chat_member.cleared_history_at IS NULL
			OR chat_message.created_at > chat_member.cleared_history_at
		)
	ORDER BY chat_message.created_at DESC, chat_message.message_id
	LIMIT @limit
	;`

	routeParams := &GetChatMessageRouteParams{}
	err := c.ParamsParser(routeParams)
	if err != nil {
		return err
	}

	queryParams := &GetChatMessagesQueryParams{}
	err = c.QueryParser(queryParams)
	if err != nil {
		return err
	}

	userId := 0
	wallet := api.getAuthedWallet(c)
	userId, err = api.getUserIDFromWallet(c.Context(), wallet)
	if err != nil {
		return err
	}

	beforeCursorPos := time.Now().UTC()
	afterCursorPos := time.Time{}
	if queryParams.Before != nil {
		beforeCursorPos = *queryParams.Before
	}
	if queryParams.After != nil {
		afterCursorPos = *queryParams.After
	}

	limit := 50
	if queryParams.Limit != 0 {
		limit = queryParams.Limit
	}

	rawRows, err := api.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id": userId,
		"chat_id": routeParams.ChatID,
		"before":  beforeCursorPos,
		"after":   afterCursorPos,
		"limit":   limit,
	})
	if err != nil {
		return err
	}

	rows, err := pgx.CollectRows(rawRows, pgx.RowToStructByName[dbv1.ChatMessageAndReactionsRow])
	if err != nil {
		return err
	}

	return c.JSON(CommsResponse{
		Data: rows,
		Health: CommsHealth{
			IsHealthy: true,
		},
	})
}
