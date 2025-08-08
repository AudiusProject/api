package dbv1

import (
	"context"
	"fmt"
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5"
)

type GetTipsParams struct {
	MyId                 int32     `json:"my_id"`
	Limit                int       `json:"limit"`
	Offset               int       `json:"offset"`
	UserId               int32     `json:"user_id"`
	ReceiverMinFollowers int       `json:"receiver_min_followers"`
	ReceiverIsVerified   bool      `json:"receiver_is_verified"`
	CurrentUserFollows   *string   `json:"current_user_follows"`
	UniqueBy             *string   `json:"unique_by"`
	MinSlot              *int      `json:"min_slot"`
	MaxSlot              *int      `json:"max_slot"`
	TxSignatures         *[]string `json:"tx_signatures"`
	ExcludeRecipients    []int32   `json:"exclude_recipients"`
}

type Supporter struct {
	UserID trashid.HashId `json:"user_id"`
}

type fullTipRow struct {
	Amount             int64          `json:"amount"`
	SenderUserId       trashid.HashId `json:"sender_user_id"`
	ReceiverUserId     trashid.HashId `json:"receiver_user_id"`
	Slot               int64          `json:"slot"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	FolloweeSupporters []int32        `json:"followee_supporters"`
	TxSignature        string         `json:"tx_signature"`
}

type FullTip struct {
	Amount             string      `json:"amount"`
	Sender             FullUser    `json:"sender"`
	Receiver           FullUser    `json:"receiver"`
	CreatedAt          time.Time   `json:"created_at"`
	Slot               int64       `json:"slot"`
	FolloweeSupporters []Supporter `json:"followee_supporters"`
	TxSignature        string      `json:"tx_signature"`
}

type MinTip struct {
	Amount    string    `json:"amount"`
	Sender    MinUser   `json:"sender"`
	Receiver  MinUser   `json:"receiver"`
	CreatedAt time.Time `json:"created_at"`
}

func ToMinTip(fullTip FullTip) MinTip {
	return MinTip{
		Amount:    fullTip.Amount,
		Sender:    ToMinUser(fullTip.Sender),
		Receiver:  ToMinUser(fullTip.Receiver),
		CreatedAt: fullTip.CreatedAt,
	}
}

func ToMinTips(fullTips []FullTip) []MinTip {
	result := make([]MinTip, len(fullTips))
	for i, tip := range fullTips {
		result[i] = ToMinTip(tip)
	}
	return result
}

func (q *Queries) FullTips(ctx context.Context, arg GetTipsParams) ([]FullTip, error) {
	baseSQL := `
	WITH followees AS (
		SELECT followee_user_id
		FROM follows
		WHERE is_current = true
			AND is_delete = false
			AND follower_user_id = @user_id
	),
	tips AS (
		SELECT 
			ut.signature,
			ut.slot,
			ut.sender_user_id,
			ut.receiver_user_id,
			ut.amount,
			ut.created_at,
			ut.updated_at
		FROM user_tips ut`

	var joins []string
	var conditions []string

	// Filtering on receiver
	if arg.ReceiverMinFollowers > 0 {
		joins = append(joins, `
			JOIN aggregate_user au ON au.user_id = ut.receiver_user_id`)
		conditions = append(conditions, "au.follower_count >= @receiver_min_followers")
	}
	if arg.ReceiverIsVerified {
		joins = append(joins, `
			JOIN users u ON u.user_id = ut.receiver_user_id`)
		conditions = append(conditions, "u.is_current = true AND u.is_verified = true")
	}

	// Filtering on followee
	if arg.UserId != 0 && arg.CurrentUserFollows != nil {
		switch *arg.CurrentUserFollows {
		case "receiver":
			joins = append(joins, `
				JOIN followees f_recv
				  ON ut.receiver_user_id = f_recv.followee_user_id`)
		case "sender":
			joins = append(joins, `
				JOIN followees f_send
				  ON ut.sender_user_id = f_send.followee_user_id`)
		case "sender_or_receiver":
			joins = append(joins,
				`LEFT JOIN followees f_send
				   ON ut.sender_user_id = f_send.followee_user_id`,
				`LEFT JOIN followees f_recv
				   ON ut.receiver_user_id = f_recv.followee_user_id`,
			)
			conditions = append(conditions,
				"(f_send.followee_user_id IS NOT NULL OR f_recv.followee_user_id IS NOT NULL)",
			)
		}
	}

	// Filtering on slot
	if arg.MinSlot != nil && *arg.MinSlot > 0 {
		conditions = append(conditions,
			"ut.slot >= @min_slot",
		)
	}
	if arg.MaxSlot != nil && *arg.MaxSlot > 0 {
		conditions = append(conditions,
			"ut.slot <= @max_slot",
		)
	}

	// Filtering on recipient
	if len(arg.ExcludeRecipients) > 0 {
		conditions = append(conditions,
			"ut.receiver_user_id != ALL(@exclude_recipients::int[])",
		)
	}

	// Filtering on tx signature
	if arg.TxSignatures != nil && len(*arg.TxSignatures) > 0 {
		conditions = append(conditions,
			"ut.signature = ANY(@tx_signatures::text[])",
		)
	}

	// Construct the full query
	sql := baseSQL
	for _, join := range joins {
		sql += " " + join
	}

	if len(conditions) > 0 {
		sql += " WHERE "
		for i, condition := range conditions {
			if i > 0 {
				sql += " AND "
			}
			sql += condition
		}
	}

	sql += `
		ORDER BY ut.slot DESC
		LIMIT @limit OFFSET @offset
	)`

	// Add followee tippers CTE if user_id is provided
	if arg.UserId != 0 {
		sql += `,
	followee_tippers AS (
		SELECT 
			aut.sender_user_id,
			aut.receiver_user_id,
			f.followee_user_id
		FROM followees f
		LEFT JOIN aggregate_user_tips aut ON aut.sender_user_id = f.followee_user_id
	)`
	}

	sql += `
	SELECT 
		t.signature as tx_signature,
		t.slot,
		t.sender_user_id,
		t.receiver_user_id,
		t.amount,
		t.created_at,
		t.updated_at`

	if arg.UserId != 0 {
		sql += `,
		COALESCE(
			array_agg(ft.sender_user_id) FILTER (WHERE ft.sender_user_id IS NOT NULL), 
			'{}'::int[]
		) as followee_supporters`
	} else {
		sql += `,
		'{}'::int[] as followee_supporters`
	}

	sql += `
		FROM tips t`
	if arg.UserId != 0 {
		sql += `
		LEFT JOIN followee_tippers ft ON ft.receiver_user_id = t.receiver_user_id`
	}
	sql += `
		GROUP BY 
			t.signature, 
			t.slot, 
			t.sender_user_id, 
			t.receiver_user_id, 
			t.amount, 
			t.created_at, 
			t.updated_at
		ORDER BY t.slot DESC`

	rows, err := q.db.Query(ctx, sql, pgx.NamedArgs{
		"user_id":                arg.UserId,
		"receiver_min_followers": arg.ReceiverMinFollowers,
		"min_slot":               arg.MinSlot,
		"max_slot":               arg.MaxSlot,
		"limit":                  arg.Limit,
		"offset":                 arg.Offset,
		"exclude_recipients":     arg.ExcludeRecipients,
		"tx_signatures":          arg.TxSignatures,
	})
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tipRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[fullTipRow])
	if err != nil {
		return nil, err
	}

	// Collect all users
	userIdSet := make(map[trashid.HashId]struct{})
	for _, row := range tipRows {
		userIdSet[row.SenderUserId] = struct{}{}
		userIdSet[row.ReceiverUserId] = struct{}{}
	}
	userIds := make([]int32, 0, len(userIdSet))
	for id := range userIdSet {
		userIds = append(userIds, int32(id))
	}
	userMap, err := q.FullUsersKeyed(ctx, GetUsersParams{
		MyID: arg.MyId,
		Ids:  userIds,
	})
	if err != nil {
		return nil, err
	}

	// Attach users to tips
	tips := make([]FullTip, len(tipRows))
	for i, row := range tipRows {
		supporters := []Supporter{}
		if len(row.FolloweeSupporters) > 0 {
			for _, supporter := range row.FolloweeSupporters {
				supporters = append(supporters, Supporter{
					UserID: trashid.HashId(supporter),
				})
			}
		}

		sender, _ := userMap[int32(row.SenderUserId)]
		receiver, _ := userMap[int32(row.ReceiverUserId)]

		tips[i] = FullTip{
			Amount:             fmt.Sprintf("%d", row.Amount/1e8*1e18),
			Sender:             sender,
			Receiver:           receiver,
			Slot:               row.Slot,
			CreatedAt:          row.CreatedAt,
			FolloweeSupporters: supporters,
			TxSignature:        row.TxSignature,
		}
	}

	return tips, nil
}
