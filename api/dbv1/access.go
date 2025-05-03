package dbv1

import (
	"context"
)

type Access struct {
	Stream   bool `json:"stream"`
	Download bool `json:"download"`
}

func (q *Queries) GetTrackAccess(
	ctx context.Context,
	myId int32,
	conditions *AccessGate,
	track *GetTracksRow,
	user *FullUser,
) bool {
	// No track? no access
	if track == nil || user == nil {
		return false
	}

	// No conditions means open access
	if conditions == nil {
		return true
	}

	// No myId? no access. we need to know who you are if there are conditions.
	if myId == 0 {
		return false
	}

	// You always have access to your own content
	if myId == user.UserID {
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

	case conditions.UsdcPurchase != nil:
		// Purchased the track directly
		var hasPurchased bool
		err := q.db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM usdc_purchases
				WHERE buyer_user_id = $1
				AND content_id = $2
				AND seller_user_id = $3
				AND content_type = 'track'
			)
		`, myId, track.TrackID, user.ID).Scan(&hasPurchased)

		if err != nil {
			return false
		}

		if hasPurchased {
			return true
		}

		// Purchased an album containing the track
		if len(track.PlaylistsContainingTrack) > 0 {
			var albumPurchaseExists bool
			err := q.db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1
					FROM usdc_purchases
					WHERE buyer_user_id = $1
					AND content_id = ANY($2)
					AND content_type = 'album'
				)
			`, myId, track.PlaylistsContainingTrack).Scan(&albumPurchaseExists)

			if err != nil {
				return false
			}

			if albumPurchaseExists {
				return true
			}
		}

		// Purchased an album containing the track before it was removed
		if len(track.PlaylistsPreviouslyContainingTrack) > 0 {
			var hasPreviousAlbumPurchase bool
			err := q.db.QueryRow(ctx, `
				SELECT EXISTS (
					SELECT 1
					FROM usdc_purchases up
					JOIN jsonb_each_text($2) AS prev_playlists(playlist_id, removal_time)
					ON up.content_id = prev_playlists.playlist_id::integer
					WHERE up.buyer_user_id = $1
					AND up.content_type = 'album'
					AND up.created_at <= to_timestamp(prev_playlists.removal_time::numeric)
				)
			`, myId, track.PlaylistsPreviouslyContainingTrack).Scan(&hasPreviousAlbumPurchase)

			if err != nil {
				return false
			}

			if hasPreviousAlbumPurchase {
				return true
			}
		}
	}

	return false
}

func (q *Queries) GetPlaylistAccess(
	ctx context.Context,
	myId int32,
	conditions *AccessGate,
	playlist *GetPlaylistsRow,
	user *FullUser,
) bool {
	// No playlist? no access.
	if playlist == nil || user == nil {
		return false
	}

	// no conditions means open access
	if conditions == nil {
		return true
	}

	// I always have access to my own content
	if myId != 0 && myId == user.UserID {
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

	case conditions.UsdcPurchase != nil:
		// Purchased the album directly
		var hasPurchased bool
		err := q.db.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1
				FROM usdc_purchases
				WHERE buyer_user_id = $1
				AND content_id = $2
				AND content_type = 'album'
			)
		`, myId, playlist.PlaylistID).Scan(&hasPurchased)

		if err != nil {
			return false
		}

		return hasPurchased
	}

	return false
}
