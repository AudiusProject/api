package comms

import (
	"context"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func chatCreate(db dbv1.DBTX, ctx context.Context, userId int32, ts time.Time, params ChatCreateRPCParams) error {
	var err error

	// first find any blasts that should seed this chat ...
	var blasts []blastRow
	for _, invite := range params.Invites {
		invitedUserId, err := trashid.DecodeHashId(invite.UserID)
		if err != nil {
			return err
		}

		pending, err := getNewBlasts(db, context.Background(), getNewBlastsParams{
			UserID: int32(invitedUserId),
			ChatID: params.ChatID,
		})
		if err != nil {
			return err
		}
		blasts = append(blasts, pending...)
	}

	// it is possible that two conflicting chats get created at the same time
	// in which case there will be two different chat secrets
	// to deterministically resolve this, if there is a conflict
	// we keep the chat with the earliest relayed_at (created_at) timestamp
	_, err = db.Exec(ctx, `
		insert into chat
			(chat_id, created_at, last_message_at)
		values
			($1, $2, $2)
		on conflict (chat_id)
		do update set created_at = $2, last_message_at = $2 where chat.created_at > $2
		`, params.ChatID, ts.UTC())
	if err != nil {
		return err
	}

	for _, invite := range params.Invites {

		invitedUserId, err := trashid.DecodeHashId(invite.UserID)
		if err != nil {
			return err
		}

		// similar to above... if there is a conflict when creating chat_member records
		// keep the version with the earliest relayed_at (created_at) timestamp.
		_, err = db.Exec(ctx, `
		insert into chat_member
			(chat_id, invited_by_user_id, invite_code, user_id, created_at)
		values
			($1, $2, $3, $4, $5)
		on conflict (chat_id, user_id)
		do update set invited_by_user_id=$2, invite_code=$3, created_at=$5 where chat_member.created_at > $5`,
			params.ChatID, userId, invite.InviteCode, invitedUserId, ts.UTC())
		if err != nil {
			return err
		}

	}

	for _, blast := range blasts {
		_, err = db.Exec(ctx, `
		insert into chat_message
			(message_id, chat_id, user_id, created_at, blast_id)
		values
			($1, $2, $3, $4, $5)
		on conflict do nothing
		`, trashid.BlastMessageID(blast.BlastID, params.ChatID), params.ChatID, blast.FromUserID, blast.CreatedAt.UTC(), blast.BlastID)
		if err != nil {
			return err
		}
	}

	err = chatUpdateLatestFields(db, ctx, params.ChatID)

	return err
}

func chatDelete(db dbv1.DBTX, ctx context.Context, userId int32, chatId string, messageTimestamp time.Time) error {
	_, err := db.Exec(ctx, "update chat_member set cleared_history_at = $1, last_active_at = $1, unread_count = 0, is_hidden = true where chat_id = $2 and user_id = $3", messageTimestamp.UTC(), chatId, userId)
	return err
}

func chatUpdateLatestFields(db dbv1.DBTX, ctx context.Context, chatId string) error {
	// universal latest message thing
	_, err := db.Exec(ctx, `
	with latest as (
		select
			m.chat_id,
			m.created_at,
			m.ciphertext,
			m.blast_id,
			b.plaintext
		from
			chat_message m
			left join chat_blast b using (blast_id)
		where m.chat_id = $1
		order by m.created_at desc
		limit 1
	)
	update chat c
	set
		last_message_at = latest.created_at,
		last_message = coalesce(latest.ciphertext, latest.plaintext),
		last_message_is_plaintext = latest.blast_id is not null
	from latest
	where c.chat_id = latest.chat_id;
	`, chatId)
	if err != nil {
		return err
	}

	// set chat_member.is_hidden to false
	// if there are any non-blast messages, reactions,
	// or any blasts from the other party after cleared_history_at
	_, err = db.Exec(ctx, `
	UPDATE chat_member member
	SET is_hidden = NOT EXISTS(

		-- Check for chat messages
		SELECT msg.message_id
		FROM chat_message msg
		LEFT JOIN chat_blast b USING (blast_id)
		WHERE msg.chat_id = member.chat_id
		AND (cleared_history_at IS NULL OR msg.created_at > cleared_history_at)
		AND (msg.blast_id IS NULL OR b.from_user_id != member.user_id)

		UNION

		-- Check for chat message reactions
		SELECT r.message_id
		FROM chat_message_reactions r
		LEFT JOIN chat_message msg ON r.message_id = msg.message_id
		WHERE msg.chat_id = member.chat_id
		AND r.user_id != member.user_id
		AND (cleared_history_at IS NULL OR (r.created_at > cleared_history_at AND msg.created_at > cleared_history_at))
	),
	unread_count = (
		select count(*)
		from chat_message msg
		where msg.created_at > COALESCE(member.last_active_at, '1970-01-01'::timestamp)
		and msg.user_id != member.user_id
		and msg.chat_id = member.chat_id
	)
	WHERE member.chat_id = $1
	`, chatId)
	return err
}

func chatSendMessage(db dbv1.DBTX, ctx context.Context, userId int32, chatId string, messageId string, messageTimestamp time.Time, ciphertext string) error {
	var err error

	_, err = db.Exec(ctx, "insert into chat_message (message_id, chat_id, user_id, created_at, ciphertext) values ($1, $2, $3, $4, $5)",
		messageId, chatId, userId, messageTimestamp.UTC(), ciphertext)
	if err != nil {
		return err
	}

	// update chat's info on last message
	err = chatUpdateLatestFields(db, ctx, chatId)
	if err != nil {
		return err
	}

	// sending a message implicitly marks activity for sender...
	err = chatReadMessages(db, ctx, userId, chatId, messageTimestamp)
	if err != nil {
		return err
	}

	return err
}

func chatReactMessage(db dbv1.DBTX, ctx context.Context, userId int32, chatId string, messageId string, reaction *string, messageTimestamp time.Time) error {
	var err error
	if reaction != nil {
		_, err = db.Exec(ctx, `
		insert into chat_message_reactions
			(user_id, message_id, reaction, created_at, updated_at)
		values
			($1, $2, $3, $4, $4)
		on conflict (user_id, message_id)
		do update set reaction = $3, updated_at = $4 where chat_message_reactions.updated_at < $4`,
			userId, messageId, *reaction, messageTimestamp.UTC())
	} else {
		_, err = db.Exec(ctx, "delete from chat_message_reactions where user_id = $1 and message_id = $2 and updated_at < $3", userId, messageId, messageTimestamp.UTC())
	}
	if err != nil {
		return err
	}

	// update chat's info on reaction
	err = chatUpdateLatestFields(db, ctx, chatId)
	return err
}

func chatReadMessages(db dbv1.DBTX, ctx context.Context, userId int32, chatId string, readTimestamp time.Time) error {
	_, err := db.Exec(ctx, "update chat_member set unread_count = 0, last_active_at = $1 where chat_id = $2 and user_id = $3",
		readTimestamp.UTC(), chatId, userId)
	return err
}

var permissions = []ChatPermission{
	ChatPermissionFollowees,
	ChatPermissionFollowers,
	ChatPermissionTippees,
	ChatPermissionTippers,
	ChatPermissionVerified,
}

// Helper function to check if a permit is in the permitList
func isInPermitList(permit ChatPermission, permitList []ChatPermission) bool {
	for _, p := range permitList {
		if p == permit {
			return true
		}
	}
	return false
}

func updatePermissions(db dbv1.DBTX, ctx context.Context, userId int32, permit ChatPermission, permitAllowed bool, messageTimestamp time.Time) error {
	_, err := db.Exec(ctx, `
    insert into chat_permissions (user_id, permits, allowed, updated_at)
    values ($1, $2, $3, $4)
    on conflict (user_id, permits)
    do update set allowed = $3 where chat_permissions.updated_at < $4
    `, userId, permit, permitAllowed, messageTimestamp.UTC())
	return err
}

func chatSetPermissions(db dbv1.DBTX, ctx context.Context, userId int32, permits ChatPermission, permitList []ChatPermission, allow *bool, messageTimestamp time.Time) error {

	// if "all" or "none" or is singular permission style (allow == nil) delete any old rows
	if allow == nil || permits == ChatPermissionAll || permits == ChatPermissionNone || isInPermitList(ChatPermissionAll, permitList) || isInPermitList(ChatPermissionNone, permitList) {
		_, err := db.Exec(ctx, `
			delete from chat_permissions where user_id = $1 and updated_at < $2
		`, userId, messageTimestamp.UTC())
		if err != nil {
			return err
		}
	}

	// old: singular permission style
	if allow == nil {
		// insert
		_, err := db.Exec(ctx, `
		insert into chat_permissions (user_id, permits, updated_at)
		values ($1, $2, $3)
		on conflict do nothing`, userId, permits, messageTimestamp.UTC())
		return err
	}

	// Special case for "all" and "none" - no other rows should be inserted
	if isInPermitList(ChatPermissionAll, permitList) {
		err := updatePermissions(db, ctx, userId, ChatPermissionAll, true, messageTimestamp)
		return err
	} else if isInPermitList(ChatPermissionNone, permitList) {
		err := updatePermissions(db, ctx, userId, ChatPermissionNone, true, messageTimestamp)
		return err
	}

	// new: multiple (checkbox) permission style
	for _, permit := range permissions {
		permitAllowed := isInPermitList(permit, permitList)
		err := updatePermissions(db, ctx, userId, permit, permitAllowed, messageTimestamp)
		if err != nil {
			return err
		}
	}
	return nil
}

func chatBlock(db dbv1.DBTX, ctx context.Context, userId int32, blockeeUserId int32, messageTimestamp time.Time) error {
	_, err := db.Exec(ctx, "insert into chat_blocked_users (blocker_user_id, blockee_user_id, created_at) values ($1, $2, $3) on conflict do nothing", userId, blockeeUserId, messageTimestamp.UTC())
	return err
}

func chatUnblock(db dbv1.DBTX, ctx context.Context, userId int32, unblockedUserId int32, messageTimestamp time.Time) error {
	_, err := db.Exec(ctx, "delete from chat_blocked_users where blocker_user_id = $1 and blockee_user_id = $2 and created_at < $3", userId, unblockedUserId, messageTimestamp.UTC())
	return err
}

type blastRow struct {
	PendingChatID            string      `db:"-" json:"pending_chat_id"`
	BlastID                  string      `db:"blast_id" json:"blast_id"`
	FromUserID               int32       `db:"from_user_id" json:"from_user_id"`
	Audience                 string      `db:"audience" json:"audience"`
	AudienceContentType      pgtype.Text `db:"audience_content_type" json:"audience_content_type"`
	AudienceContentID        pgtype.Int4 `db:"audience_content_id" json:"-"`
	AudienceContentIDEncoded pgtype.Text `db:"-" json:"audience_content_id"`
	Plaintext                string      `db:"plaintext" json:"plaintext"`
	CreatedAt                time.Time   `db:"created_at" json:"created_at"`
}

type getNewBlastsParams struct {
	UserID int32  `db:"user_id" json:"user_id"`
	ChatID string `db:"chat_id" json:"chat_id"`
}

// Helper function to get new blasts as potential chat seeds for creating a chat
func getNewBlasts(tx dbv1.DBTX, ctx context.Context, arg getNewBlastsParams) ([]blastRow, error) {

	// this query is to find new blasts for the current user
	// which don't already have a existing chat.
	// see also: subtly different inverse query exists in chat_blast.go
	// to fan out messages to existing chat
	var findNewBlasts = `
	with
	last_permission_change as (
		select max(t) as t from (
			select updated_at as t from chat_permissions where user_id = $1
			union
			select created_at as t from chat_blocked_users where blocker_user_id = $1
			union
			select to_timestamp(0)
		) as timestamp_subquery
	),
	all_new as (
		select *
		from chat_blast blast
		where
		from_user_id in (
			-- follower_audience
			SELECT followee_user_id AS from_user_id
			FROM follows
			WHERE blast.audience = 'follower_audience'
				AND follows.followee_user_id = blast.from_user_id
				AND follows.follower_user_id = $1
				AND follows.is_delete = false
				AND follows.created_at < blast.created_at
		)
		OR from_user_id in (
			-- tipper_audience
			SELECT receiver_user_id
			FROM user_tips tip
			WHERE blast.audience = 'tipper_audience'
			AND receiver_user_id = blast.from_user_id
			AND sender_user_id = $1
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
				AND t.owner_id = $1
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
				AND p.buyer_user_id = $1
				AND (
					audience_content_id IS NULL
					OR (
						blast.audience_content_type = p.content_type::text
						AND blast.audience_content_id = p.content_id
					)
				)
		)
		OR from_user_id IN (
			-- coin_holder_audience via sol_user_balances
			SELECT ac.user_id
			FROM artist_coins ac
			JOIN sol_user_balances sub ON sub.mint = ac.mint
			WHERE blast.audience = 'coin_holder_audience'
				AND ac.user_id = blast.from_user_id
				AND sub.user_id = $1
				AND sub.balance > 0
				-- TODO: PE-6663 This isn't entirely correct yet, need to check "time of most recent membership"
				AND sub.created_at < blast.created_at
		)
	)
	select * from all_new
	where created_at > (select t from last_permission_change)
	and chat_allowed(from_user_id, $1)
	order by created_at
	`

	rows, err := tx.Query(ctx, findNewBlasts, arg.UserID)
	if err != nil {
		return nil, err
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[blastRow])
	if err != nil {
		return nil, err
	}

	for idx, blastRow := range items {
		chatId := trashid.ChatID(int(arg.UserID), int(blastRow.FromUserID))
		items[idx].PendingChatID = chatId

		if blastRow.AudienceContentID.Valid {
			encoded, _ := trashid.EncodeHashId(int(blastRow.AudienceContentID.Int32))
			items[idx].AudienceContentIDEncoded.String = encoded
			items[idx].AudienceContentIDEncoded.Valid = true
		}
	}

	rows, err = tx.Query(ctx, `select chat_id from chat_member where user_id = $1`, arg.UserID)
	if err != nil {
		return nil, err
	}

	existingChatIdList, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return nil, err
	}

	existingChatIds := map[string]bool{}
	for _, id := range existingChatIdList {
		existingChatIds[id] = true
	}

	// filter out blast rows where chatIds is taken
	filtered := make([]blastRow, 0, len(items))
	for _, item := range items {
		if existingChatIds[item.PendingChatID] {
			continue
		}
		// allow caller to filter to blasts for a given chat ID
		if arg.ChatID != "" && item.PendingChatID != arg.ChatID {
			continue
		}
		filtered = append(filtered, item)
	}

	return filtered, err

}
