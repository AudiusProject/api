package api

import (
	"errors"
	"strings"
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
	Limit   int        `query:"limit" default:"50" validate:"min=1,max=100"`
}

func (app *ApiServer) getChatMessages(c *fiber.Ctx) error {
	sql := `
	SELECT
		chat_message.message_id,
		chat_message.chat_id,
		chat_message.user_id,
		chat_message.created_at,
		COALESCE(chat_blast.audience, '') AS audience,
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

	sqlSummary := `
	WITH messages AS (
		SELECT
			chat_message.message_id, chat_message.created_at
		FROM chat_message
		JOIN chat_member ON chat_message.chat_id = chat_member.chat_id
		WHERE chat_member.user_id = @user_id
		AND chat_message.chat_id = @chat_id
		AND (chat_member.cleared_history_at IS NULL
			OR chat_message.created_at > chat_member.cleared_history_at)
		)
	SELECT
		(SELECT COUNT(*) AS total_count FROM messages),
		(SELECT COUNT(*) FROM messages WHERE created_at < @before) AS before_count,
		(SELECT COUNT(*) FROM messages WHERE created_at > @after) AS after_count,
		@before AS prev,
		@after AS next
	;
	`

	outgoingBlastSQL := `
	SELECT
		b.blast_id as message_id,
		@chat_id as chat_id,
		b.from_user_id as user_id,
		b.created_at,
		'' as audience,
		b.plaintext as ciphertext,
		true as is_plaintext,
		'[]'::json AS reactions
	FROM chat_blast b
	WHERE b.from_user_id = @user_id
		AND concat_ws(':', audience, audience_content_type, 
			CASE 
				WHEN audience_content_id IS NOT NULL THEN id_encode(audience_content_id)
				ELSE NULL 
			END) = @chat_id
	  AND b.created_at < @before
	  AND b.created_at > @after
	ORDER BY b.created_at DESC
	LIMIT @limit
	`

	blastSummarySQL := `
	WITH messages AS (
		SELECT
			b.blast_id as message_id, b.created_at
		FROM chat_blast b
		WHERE b.from_user_id = @user_id
			AND concat_ws(':', audience, audience_content_type, 
				CASE 
					WHEN audience_content_id IS NOT NULL THEN id_encode(audience_content_id)
					ELSE NULL 
				END) = @chat_id
		)
	SELECT
		(SELECT COUNT(*) AS total_count FROM messages),
		(SELECT COUNT(*) FROM messages WHERE created_at < @before) AS before_count,
		(SELECT COUNT(*) FROM messages WHERE created_at > @after) AS after_count,
		@before AS prev,
		@after AS next
	`

	routeParams := &GetChatMessageRouteParams{}
	err := c.ParamsParser(routeParams)
	if err != nil {
		return err
	}

	queryParams := &GetChatMessagesQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, queryParams); err != nil {
		return err
	}

	userId := 0
	wallet := app.getAuthedWallet(c)
	userId, err = app.getUserIDFromWallet(c.Context(), wallet)
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

	// Special case to handle outgoing blasts...
	if queryParams.IsBlast {
		parts := strings.Split(routeParams.ChatID, ":")
		if len(parts) < 1 {
			return errors.New("bad request: invalid blast id")
		}
		audience := parts[0]

		if audience == "follower_audience" ||
			audience == "tipper_audience" ||
			audience == "customer_audience" ||
			audience == "remixer_audience" ||
			audience == "coin_holder_audience" {

			rawRows, err := app.pool.Query(c.Context(), outgoingBlastSQL, pgx.NamedArgs{
				"user_id": userId,
				"chat_id": routeParams.ChatID,
				"before":  beforeCursorPos,
				"after":   afterCursorPos,
				"limit":   queryParams.Limit,
			})
			if err != nil {
				return err
			}

			rows, err := pgx.CollectRows(rawRows, pgx.RowToStructByName[dbv1.ChatMessageAndReactionsRow])
			if err != nil {
				return err
			}

			if len(rows) > 0 {
				beforeCursorPos = rows[len(rows)-1].CreatedAt
				afterCursorPos = rows[0].CreatedAt
			}

			// Use the same summary query but adapted for blast messages

			summaryRaw, err := app.pool.Query(c.Context(), blastSummarySQL, pgx.NamedArgs{
				"user_id": userId,
				"chat_id": routeParams.ChatID,
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
		} else {
			return errors.New("bad request: unsupported audience " + audience)
		}
	}

	rawRows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id": userId,
		"chat_id": routeParams.ChatID,
		"before":  beforeCursorPos,
		"after":   afterCursorPos,
		"limit":   queryParams.Limit,
	})
	if err != nil {
		return err
	}

	rows, err := pgx.CollectRows(rawRows, pgx.RowToStructByName[dbv1.ChatMessageAndReactionsRow])
	if err != nil {
		return err
	}

	if len(rows) > 0 {
		beforeCursorPos = rows[len(rows)-1].CreatedAt
		afterCursorPos = rows[0].CreatedAt
	}
	summaryRaw, err := app.pool.Query(c.Context(), sqlSummary, pgx.NamedArgs{
		"user_id": userId,
		"chat_id": routeParams.ChatID,
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
