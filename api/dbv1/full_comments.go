package dbv1

import (
	"context"
	"time"

	"api.audius.co/trashid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type GetCommentsParams struct {
	MyID            interface{} `json:"my_id"`
	Ids             []int32     `json:"ids"`
	IncludeUnlisted bool        `json:"include_unlisted"`
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
	IsDelete             bool        `json:"-"`
	IsMembersOnly        bool        `json:"is_members_only"`
	VideoUrl             *string     `json:"video_url"`
	IsTombstone          bool        `json:"is_tombstone"`
	ReactCount           int         `json:"react_count"`
	CreatedAt            time.Time   `json:"created_at"`
	UpdatedAt            time.Time   `json:"updated_at"`

	ReplyCount      int           `json:"reply_count"`
	Replies         []FullComment `json:"replies"`
	ParentCommentId pgtype.Int4   `json:"parent_comment_id"`

	ReplyIds []int32 `db:"reply_ids" json:"-"`
}

func (q *Queries) FullCommentsKeyed(ctx context.Context, arg GetCommentsParams) (map[int32]FullComment, error) {
	if len(arg.Ids) == 0 {
		return nil, nil
	}

	sql := `
	SELECT
		comments.comment_id AS id,
		comment_threads.parent_comment_id,
		comments.entity_type,
		comments.entity_id,
		comments.user_id,
		comments.text AS message,

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

		comments.track_timestamp_s,

		(
			SELECT count(*)
			FROM comment_reactions
			WHERE comment_id = comments.comment_id
			AND is_delete = false
		) as react_count,


		(
			SELECT array_agg(comment_id)
			FROM comment_threads
			JOIN comments cc USING (comment_id)
			WHERE parent_comment_id = comments.comment_id
			AND cc.is_delete = false
		) as reply_ids,

		comments.is_edited,

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
			AND user_id = COALESCE(tracks.owner_id, comments.entity_id)
			AND is_delete = false
		) AS is_artist_reacted,

		comments.is_delete,
		comments.is_members_only,
		comments.video_url,

		coalesce((
			SELECT is_muted
			FROM comment_notification_settings mutes
			WHERE @my_id > 0
			AND mutes.user_id = @my_id
			AND mutes.entity_type = comments.entity_type
			AND mutes.entity_id = comments.entity_id
			LIMIT 1
		), false) as is_muted,

		comments.created_at,
		comments.updated_at

	FROM comments
	LEFT JOIN tracks ON comments.entity_type = 'Track' AND comments.entity_id = tracks.track_id
	LEFT JOIN comment_threads USING (comment_id)
	WHERE comments.comment_id = ANY(@ids::int[])
	AND (
		(comments.entity_type = 'Track' AND (@include_unlisted = true OR COALESCE(tracks.is_unlisted, false) = false))
		OR comments.entity_type = 'FanClub'
	)
	ORDER BY comments.created_at DESC
	`

	rows, err := q.db.Query(ctx, sql, pgx.NamedArgs{
		"ids":              arg.Ids,
		"my_id":            arg.MyID,
		"include_unlisted": arg.IncludeUnlisted,
	})
	if err != nil {
		return nil, err
	}

	comments, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[FullComment])
	if err != nil {
		return nil, err
	}

	commentMap := map[int32]FullComment{}
	for _, comment := range comments {
		commentMap[int32(comment.Id)] = comment
	}

	replyIds := []int32{}
	for _, comment := range comments {
		replyIds = append(replyIds, comment.ReplyIds...)
	}
	replyMap, err := q.FullCommentsKeyed(ctx, GetCommentsParams{
		MyID:            arg.MyID,
		Ids:             replyIds,
		IncludeUnlisted: arg.IncludeUnlisted,
	})
	if err != nil {
		return nil, err
	}

	for id, comment := range commentMap {
		for _, replyId := range comment.ReplyIds {
			if reply, ok := replyMap[replyId]; ok {
				comment.Replies = append(comment.Replies, reply)
			}
		}
		comment.ReplyCount = len(comment.Replies)

		if comment.IsDelete {
			comment.Message = "[Removed]"
			if comment.ReplyCount > 0 {
				comment.IsTombstone = true
			}
		}
		commentMap[id] = comment
	}

	return commentMap, nil

}

func (q *Queries) FullComments(ctx context.Context, arg GetCommentsParams) ([]FullComment, error) {
	commentMap, err := q.FullCommentsKeyed(ctx, arg)
	if err != nil {
		return nil, err
	}

	comments := make([]FullComment, 0, len(arg.Ids))
	for _, id := range arg.Ids {
		if c, ok := commentMap[id]; ok {
			comments = append(comments, c)
		}
	}
	return comments, nil
}
