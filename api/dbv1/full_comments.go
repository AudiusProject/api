package dbv1

import (
	"context"
	"fmt"
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type GetCommentsParams struct {
	MyID interface{} `json:"my_id"`
	Ids  []int32     `json:"ids"`
}

type FullComment struct {
	Id         trashid.HashId `json:"id"`
	EntityType string         `json:"entity_type"`
	EntityId   trashid.HashId `json:"entity_id"`
	UserId     trashid.HashId `json:"user_id"`
	Message    string         `json:"message"`
	Mentions   []struct {
		UserId int    `json:"user_id"`
		Handle string `json:"handle"`
	} `json:"mentions"`
	TrackTimestampS      pgtype.Int4 `json:"track_timestamp_s"`
	IsMuted              bool        `json:"is_muted"`
	IsEdited             bool        `json:"is_edited"`
	IsCurrentUserReacted bool        `json:"is_current_user_reacted"`
	IsArtistReacted      bool        `json:"is_artist_reacted"`
	IsTombstone          bool        `json:"is_tombstone"`
	ReactCount           int         `json:"react_count"`
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`

	Replies []FullComment `json:"replies"`

	// this should be omitted
	ReplyIds        []int32     `db:"reply_ids" json:"-"`
	ParentCommentId pgtype.Int4 `json:"-"`
}

func (q *Queries) FullComments(ctx context.Context, arg GetCommentsParams) ([]FullComment, error) {
	if len(arg.Ids) == 0 {
		return nil, nil
	}

	sql := `
	SELECT
		comment_id as id,
		parent_comment_id,
		entity_type,
		entity_id,
		user_id,
		text as message,

		(
			SELECT json_agg(
				json_build_object(
					'user_id', m.user_id,
					'handle', handle
				)
			)
			FROM (
				SELECT user_id, handle FROM comment_mentions
				JOIN users USING (user_id)
				WHERE comment_id = comments.comment_id
			) m
		)::jsonb as mentions,

		track_timestamp_s,

		(
			SELECT count(*)
			FROM comment_reactions
			WHERE comment_id = comments.comment_id
			AND is_delete = false
		) as react_count,


		-- reply_count
		(
			SELECT array_agg(comment_id)
			FROM comment_threads
			JOIN comments cc USING (comment_id)
			WHERE parent_comment_id = comments.comment_id
			AND cc.is_delete = false
		) as reply_ids,

		is_edited,

		EXISTS (
			SELECT 1
			FROM comment_reactions
			WHERE comment_id = comments.comment_id
			AND user_id = @my_id
			AND is_delete = false
		) AS is_current_user_reacted,

		EXISTS (
			SELECT 1
			FROM comment_reactions
			WHERE comment_id = comments.comment_id
			AND user_id = tracks.owner_id
			AND is_delete = false
		) AS is_artist_reacted,

		-- is_tombstone

		coalesce((
			SELECT is_muted
			FROM comment_notification_settings mutes
			WHERE @my_id > 0
			AND mutes.user_id = @my_id
			AND mutes.entity_type = entity_type
			AND mutes.entity_id = entity_id
			LIMIT 1
		), false) as is_muted,

		comments.created_at,
		comments.updated_at

		-- replies
	FROM comments
	JOIN tracks ON entity_id = track_id
	LEFT JOIN comment_threads USING (comment_id)
	WHERE comment_id = ANY(@ids::int[])
	ORDER BY comments.created_at DESC
	`

	rows, err := q.db.Query(ctx, sql, pgx.NamedArgs{
		"ids":   arg.Ids,
		"my_id": arg.MyID,
	})
	if err != nil {
		return nil, err
	}

	comments, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[FullComment])
	if err != nil {
		return nil, err
	}

	// fetch replies
	replyIds := []int32{}
	for _, comment := range comments {
		replyIds = append(replyIds, comment.ReplyIds...)
	}
	if len(replyIds) > 0 {
		replies, err := q.FullComments(ctx, GetCommentsParams{
			MyID: arg.MyID,
			Ids:  replyIds,
		})
		if err != nil {
			return nil, err
		}

		for _, r := range replies {
			for idx, comment := range comments {
				if r.ParentCommentId.Int32 == int32(comment.Id) {
					fmt.Println("REPLY", r, r.Id, r.ParentCommentId.Int32)
					comment.Replies = append(comment.Replies, r)
				}
				comments[idx] = comment
			}
		}

		// todo: sort replies
	}

	return comments, nil

}
