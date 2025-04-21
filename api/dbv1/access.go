package dbv1

import (
	"context"
)

type Access struct {
	Stream   bool `json:"stream"`
	Download bool `json:"download"`
}

func (q *Queries) GetTrackAccess(ctx context.Context, myId int32, conditions *AccessGate, track *GetTracksRow, user *FullUser) bool {
	if track == nil || user == nil {
		return false
	}

	// no conditions means open access
	if conditions == nil {
		return true
	}

	switch {
	case conditions.FollowUserID != nil:
		return user.DoesCurrentUserFollow
	case conditions.TipUserID != nil:
		tipUserId := *conditions.TipUserID
		var hasTipped bool
		err := q.db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM aggregate_user_tips
				WHERE sender_user_id = $1
				AND receiver_user_id = $2
				AND amount >= 0
			)
		`, myId, tipUserId).Scan(&hasTipped)

		if err != nil {
			return false
		}

		return hasTipped
	}

	return false
}

func (q *Queries) GetPlaylistAccess(ctx context.Context, myId int32, conditions *AccessGate, playlist *GetPlaylistsRow, user *FullUser) bool {
	if conditions == nil {
		return true
	}

	switch {
	case conditions.FollowUserID != nil:
		return user.DoesCurrentUserFollow
	case conditions.TipUserID != nil:
		tipUserId := *conditions.TipUserID
		var hasTipped bool
		err := q.db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM aggregate_user_tips
				WHERE sender_user_id = $1
				AND receiver_user_id = $2
				AND amount >= 0
			)
		`, myId, tipUserId).Scan(&hasTipped)

		if err != nil {
			return false
		}

		return hasTipped
	}

	return false
}
