package api

import (
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type BlastRow struct {
	PendingChatID       string          `db:"-" json:"pending_chat_id"`
	BlastID             string          `db:"blast_id" json:"blast_id"`
	FromUserID          int             `db:"-" json:"from_user_id"` // should deprecate. use SenderUserID
	SenderUserID        trashid.HashId  `db:"from_user_id" json:"sender_user_id"`
	Audience            string          `db:"audience" json:"audience"`
	AudienceContentType *string         `db:"audience_content_type" json:"audience_content_type"`
	AudienceContentID   *trashid.HashId `db:"audience_content_id" json:"audience_content_id"`
	Plaintext           string          `db:"plaintext" json:"plaintext"`
	CreatedAt           time.Time       `db:"created_at" json:"created_at"`
}

func (api ApiServer) getNewBlasts(c *fiber.Ctx) error {
	sql := `
	WITH
	last_permission_change AS (
		SELECT max(t) AS t FROM (
			SELECT updated_at AS t FROM chat_permissions WHERE user_id = @user_id
			UNION
			SELECT created_at AS t FROM chat_blocked_users WHERE blocker_user_id = @user_id
			UNION
			SELECT to_timestamp(0)
		) AS timestamp_subquery
	),
	all_new AS (
		SELECT *
		FROM chat_blast blast
		WHERE
		from_user_id IN (
			-- follower_audience
			SELECT followee_user_id AS from_user_id
			FROM follows
			WHERE blast.audience = 'follower_audience'
				AND follows.followee_user_id = blast.from_user_id
				AND follows.follower_user_id = @user_id
				AND follows.is_delete = false
				AND follows.created_at < blast.created_at
		)
		OR from_user_id IN (
			-- tipper_audience
			SELECT receiver_user_id
			FROM user_tips tip
			WHERE blast.audience = 'tipper_audience'
			AND receiver_user_id = blast.from_user_id
			AND sender_user_id = @user_id
			AND tip.created_at < blast.created_at
		)
		OR from_user_id IN  (
			-- remixer_audience
			SELECT og.owner_id
			FROM tracks t
			JOIN remixes ON remixes.child_track_id = t.track_id
			JOIN tracks og ON remixes.parent_track_id = og.track_id
			WHERE blast.audience = 'remixer_audience'
				AND og.owner_id = blast.from_user_id
				AND t.owner_id = @user_id
				AND (
					blast.audience_content_id IS NULL
					OR (
						blast.audience_content_type = 'track'
						AND blast.audience_content_id = og.track_id
					)
				)
		)
		OR from_user_id IN (
			-- customer_audience
			SELECT seller_user_id
			FROM usdc_purchases p
			WHERE blast.audience = 'customer_audience'
				AND p.seller_user_id = blast.from_user_id
				AND p.buyer_user_id = @user_id
				AND (
					audience_content_id IS NULL
					OR (
						blast.audience_content_type = p.content_type::text
						AND blast.audience_content_id = p.content_id
					)
				)
		)
	)
	SELECT * FROM all_new
	WHERE created_at > (select t from last_permission_change)
	AND chat_allowed(from_user_id, @user_id)
	ORDER BY created_at
	;`

	wallet := api.getAuthedWallet(c)
	userId, err := api.getUserIDFromWallet(c.Context(), wallet)
	if err != nil {
		return err
	}

	rawRows, err := api.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id": userId,
	})
	if err != nil {
		return err
	}

	rows, err := pgx.CollectRows(rawRows, pgx.RowToStructByName[BlastRow])
	if err != nil {
		return err
	}

	for idx, row := range rows {
		rows[idx].PendingChatID = trashid.ChatID(userId, int(row.SenderUserID))
		rows[idx].FromUserID = int(row.SenderUserID)
	}

	sqlExisting := `SELECT chat_id FROM chat_member WHERE user_id = @user_id`
	allExistingChatRowsRaw, err := api.pool.Query(c.Context(), sqlExisting, pgx.NamedArgs{
		"user_id": userId,
	})
	if err != nil {
		return err
	}

	// Get all the chat IDs for chats the user currently belongs to so that they can be filtered out
	existingChatIdList, err := pgx.CollectRows(allExistingChatRowsRaw, pgx.RowTo[string])
	if err != nil {
		return err
	}

	existingChatIds := map[string]bool{}
	for _, id := range existingChatIdList {
		existingChatIds[id] = true
	}

	// filter out blast rows where chatIds is taken
	filtered := make([]BlastRow, 0)
	for _, row := range rows {
		if existingChatIds[row.PendingChatID] {
			continue
		}
		filtered = append(filtered, row)
	}

	return c.JSON(CommsResponse{
		Data: filtered,
		Health: CommsHealth{
			IsHealthy: true,
		},
	})
}
