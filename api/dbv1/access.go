package dbv1

import (
	"context"
	"encoding/json"
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

// GetBulkTrackAccess checks access for multiple tracks in bulk by grouping them by access conditions
func (q *Queries) GetBulkTrackAccess(
	ctx context.Context,
	myId int32,
	tracks []*GetTracksRow,
	users map[int32]*FullUser,
) map[int32]Access {
	if len(tracks) == 0 {
		return nil
	}

	// Initialize result map
	result := make(map[int32]Access)

	// Collect all user IDs and track IDs we need to check
	followUserIDs := make(map[int32]struct{})
	tipUserIDs := make(map[int32]struct{})
	trackIDs := make(map[int32]struct{})
	playlistIDs := make(map[int32]struct{})
	prevPlaylistData := make(map[int32][]byte) // trackID -> JSON of previous playlists
	prevPlaylistsMap := make(map[int32][]struct {
		PlaylistID  int32  `json:"playlist_id"`
		RemovalTime string `json:"removal_time"`
	})

	for _, track := range tracks {
		if track == nil {
			continue
		}

		user := users[track.UserID]
		if user == nil {
			continue
		}

		// Handle special cases first
		if track.StreamConditions == nil && track.DownloadConditions == nil {
			result[track.TrackID] = Access{
				Stream:   true,
				Download: true,
			}
			continue
		}

		if myId == user.UserID {
			result[track.TrackID] = Access{
				Stream:   true,
				Download: true,
			}
			continue
		}

		// Collect user IDs for follow and tip conditions
		if track.StreamConditions != nil {
			if track.StreamConditions.FollowUserID != nil {
				followUserIDs[int32(*track.StreamConditions.FollowUserID)] = struct{}{}
			}
			if track.StreamConditions.TipUserID != nil {
				tipUserIDs[int32(*track.StreamConditions.TipUserID)] = struct{}{}
			}
			if track.StreamConditions.UsdcPurchase != nil {
				trackIDs[track.TrackID] = struct{}{}
				for _, playlistID := range track.PlaylistsContainingTrack {
					playlistIDs[playlistID] = struct{}{}
				}
				if len(track.PlaylistsPreviouslyContainingTrack) > 0 {
					prevPlaylistData[track.TrackID] = track.PlaylistsPreviouslyContainingTrack
					var prevPlaylists []struct {
						PlaylistID  int32  `json:"playlist_id"`
						RemovalTime string `json:"removal_time"`
					}
					if err := json.Unmarshal(track.PlaylistsPreviouslyContainingTrack, &prevPlaylists); err == nil {
						prevPlaylistsMap[track.TrackID] = prevPlaylists
					}
				}
			}
		}

		if track.DownloadConditions != nil {
			if track.DownloadConditions.FollowUserID != nil {
				followUserIDs[int32(*track.DownloadConditions.FollowUserID)] = struct{}{}
			}
			if track.DownloadConditions.TipUserID != nil {
				tipUserIDs[int32(*track.DownloadConditions.TipUserID)] = struct{}{}
			}
			if track.DownloadConditions.UsdcPurchase != nil {
				trackIDs[track.TrackID] = struct{}{}
				for _, playlistID := range track.PlaylistsContainingTrack {
					playlistIDs[playlistID] = struct{}{}
				}
				if len(track.PlaylistsPreviouslyContainingTrack) > 0 {
					prevPlaylistData[track.TrackID] = track.PlaylistsPreviouslyContainingTrack
					var prevPlaylists []struct {
						PlaylistID  int32  `json:"playlist_id"`
						RemovalTime string `json:"removal_time"`
					}
					if err := json.Unmarshal(track.PlaylistsPreviouslyContainingTrack, &prevPlaylists); err == nil {
						prevPlaylistsMap[track.TrackID] = prevPlaylists
					}
				}
			}
		}
	}

	// Convert maps to slices for queries
	followUserIDsSlice := make([]int32, 0, len(followUserIDs))
	for id := range followUserIDs {
		followUserIDsSlice = append(followUserIDsSlice, id)
	}

	tipUserIDsSlice := make([]int32, 0, len(tipUserIDs))
	for id := range tipUserIDs {
		tipUserIDsSlice = append(tipUserIDsSlice, id)
	}

	trackIDsSlice := make([]int32, 0, len(trackIDs))
	for id := range trackIDs {
		trackIDsSlice = append(trackIDsSlice, id)
	}

	playlistIDsSlice := make([]int32, 0, len(playlistIDs))
	for id := range playlistIDs {
		playlistIDsSlice = append(playlistIDsSlice, id)
	}

	// Query for followed users
	followedUsers := make(map[int32]bool)
	if len(followUserIDsSlice) > 0 {
		rows, err := q.db.Query(ctx, `
			SELECT user_id
			FROM follows
			WHERE follower_user_id = $1
			AND user_id = ANY($2)
		`, myId, followUserIDsSlice)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var userID int32
				if err := rows.Scan(&userID); err == nil {
					followedUsers[userID] = true
				}
			}
		}
	}

	// Query for tipped users
	tippedUsers := make(map[int32]bool)
	if len(tipUserIDsSlice) > 0 {
		rows, err := q.db.Query(ctx, `
			SELECT DISTINCT receiver_user_id
			FROM aggregate_user_tips
			WHERE sender_user_id = $1
			AND receiver_user_id = ANY($2)
			AND amount >= 0
		`, myId, tipUserIDsSlice)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var userID int32
				if err := rows.Scan(&userID); err == nil {
					tippedUsers[userID] = true
				}
			}
		}
	}

	// Query for purchased tracks
	purchasedTracks := make(map[int32]bool)
	if len(trackIDsSlice) > 0 {
		rows, err := q.db.Query(ctx, `
			SELECT content_id
			FROM usdc_purchases
			WHERE buyer_user_id = $1
			AND content_id = ANY($2)
			AND content_type = 'track'
		`, myId, trackIDsSlice)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var trackID int32
				if err := rows.Scan(&trackID); err == nil {
					purchasedTracks[trackID] = true
				}
			}
		}
	}

	// Query for purchased playlists
	purchasedPlaylists := make(map[int32]bool)
	if len(playlistIDsSlice) > 0 {
		rows, err := q.db.Query(ctx, `
			SELECT content_id
			FROM usdc_purchases
			WHERE buyer_user_id = $1
			AND content_id = ANY($2)
			AND content_type = 'album'
		`, myId, playlistIDsSlice)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var playlistID int32
				if err := rows.Scan(&playlistID); err == nil {
					purchasedPlaylists[playlistID] = true
				}
			}
		}
	}

	// Query for previously purchased playlists
	prevPurchasedPlaylists := make(map[int32]bool)
	if len(prevPlaylistData) > 0 {
		// Collect all previous playlist IDs
		prevPlaylistIDs := make([]int32, 0)
		for _, prevPlaylists := range prevPlaylistsMap {
			for _, prevPlaylist := range prevPlaylists {
				prevPlaylistIDs = append(prevPlaylistIDs, prevPlaylist.PlaylistID)
			}
		}

		if len(prevPlaylistIDs) > 0 {
			rows, err := q.db.Query(ctx, `
				SELECT up.content_id
				FROM usdc_purchases up
				JOIN jsonb_each_text($2) AS prev_playlists(playlist_id, removal_time)
				ON up.content_id = prev_playlists.playlist_id::integer
				WHERE up.buyer_user_id = $1
				AND up.content_type = 'album'
				AND up.content_id = ANY($3)
				AND up.created_at <= to_timestamp(prev_playlists.removal_time::numeric)
			`, myId, prevPlaylistData, prevPlaylistIDs)
			if err == nil {
				defer rows.Close()
				for rows.Next() {
					var playlistID int32
					if err := rows.Scan(&playlistID); err == nil {
						prevPurchasedPlaylists[playlistID] = true
					}
				}
			}
		}
	}

	// Now determine access for each track
	for _, track := range tracks {
		if track == nil {
			continue
		}

		user := users[track.UserID]
		if user == nil {
			result[track.TrackID] = Access{
				Stream:   false,
				Download: false,
			}
			continue
		}

		// Skip tracks we've already handled (no conditions or own content)
		if _, exists := result[track.TrackID]; exists {
			continue
		}

		access := Access{}

		// Check download access
		if track.DownloadConditions != nil {
			switch {
			case track.DownloadConditions.FollowUserID != nil:
				access.Download = followedUsers[int32(*track.DownloadConditions.FollowUserID)]
			case track.DownloadConditions.TipUserID != nil:
				access.Download = tippedUsers[int32(*track.DownloadConditions.TipUserID)]
			case track.DownloadConditions.UsdcPurchase != nil:
				// Check direct purchase
				access.Download = purchasedTracks[track.TrackID]

				// Check current playlist purchases
				if !access.Download {
					for _, playlistID := range track.PlaylistsContainingTrack {
						if purchasedPlaylists[playlistID] {
							access.Download = true
							break
						}
					}
				}

				// Check previous playlist purchases
				if !access.Download && len(track.PlaylistsPreviouslyContainingTrack) > 0 {
					for _, prevPlaylist := range prevPlaylistsMap[track.TrackID] {
						if prevPurchasedPlaylists[prevPlaylist.PlaylistID] {
							access.Download = true
							break
						}
					}
				}
			}
		}

		// If you can download it, you can stream it
		if access.Download {
			access.Stream = true
		}

		// Check stream access
		if !access.Stream && track.StreamConditions != nil {
			switch {
			case track.StreamConditions.FollowUserID != nil:
				access.Stream = followedUsers[int32(*track.StreamConditions.FollowUserID)]
			case track.StreamConditions.TipUserID != nil:
				access.Stream = tippedUsers[int32(*track.StreamConditions.TipUserID)]
			case track.StreamConditions.UsdcPurchase != nil:
				// Check direct purchase
				access.Stream = purchasedTracks[track.TrackID]

				// Check current playlist purchases
				if !access.Stream {
					for _, playlistID := range track.PlaylistsContainingTrack {
						if purchasedPlaylists[playlistID] {
							access.Stream = true
							break
						}
					}
				}

				// Check previous playlist purchases
				if !access.Stream && len(track.PlaylistsPreviouslyContainingTrack) > 0 {
					for _, prevPlaylist := range prevPlaylistsMap[track.TrackID] {
						if prevPurchasedPlaylists[prevPlaylist.PlaylistID] {
							access.Stream = true
							break
						}
					}
				}
			}
		}

		result[track.TrackID] = access
	}

	return result
}
