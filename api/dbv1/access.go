package dbv1

import (
	"context"
	"encoding/json"
	"math"
	"strconv"

	"golang.org/x/sync/errgroup"
)

type Access struct {
	Stream   bool `json:"stream"`
	Download bool `json:"download"`
}

// trackRemoval is one track's departure from one album. Buying the album
// before this moment keeps access to the track afterwards, so the pair and the
// timestamp have to travel together: two tracks can leave the same album on
// different days, and a purchase between those days covers only one of them.
type trackRemoval struct {
	TrackID     int32 `json:"track_id"`
	PlaylistID  int32 `json:"playlist_id"`
	RemovalTime int64 `json:"removal_time"`
}

// parseTrackRemovals reads tracks.playlists_previously_containing_track.
//
// The indexer writes it as a jsonb object keyed by playlist id, with the
// removal recorded as a unix timestamp under "time":
//
//	{"1284768821": {"time": 1725873897}}
//
// Anything unparseable yields no removals, which denies access rather than
// granting it.
func parseTrackRemovals(trackID int32, raw json.RawMessage) []trackRemoval {
	if len(raw) == 0 {
		return nil
	}
	var record map[string]struct {
		Time int64 `json:"time"`
	}
	if err := json.Unmarshal(raw, &record); err != nil {
		return nil
	}
	removals := make([]trackRemoval, 0, len(record))
	for playlistID, entry := range record {
		id, err := strconv.ParseInt(playlistID, 10, 32)
		if err != nil {
			continue
		}
		removals = append(removals, trackRemoval{
			TrackID:     trackID,
			PlaylistID:  int32(id),
			RemovalTime: entry.Time,
		})
	}
	return removals
}

func (q *Queries) GetPlaylistAccess(
	ctx context.Context,
	myId int32,
	conditions *AccessGate,
	playlist *GetPlaylistsRow,
	user *User,
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
				FROM v_usdc_purchases
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
	users map[int32]*User,
	solanaWallet string,
) (map[int32]Access, error) {
	// Initialize result map
	result := make(map[int32]Access)

	if len(tracks) == 0 {
		return result, nil
	}

	// Collect all user IDs and track IDs we need to check
	followUserIDs := make(map[int32]struct{})
	tipUserIDs := make(map[int32]struct{})
	trackIDs := make(map[int32]struct{})
	playlistIDs := make(map[int32]struct{})
	tokenGateTokenMints := make(map[string]struct{})
	// trackID -> the albums this track has left, and when
	prevRemovals := make(map[int32][]trackRemoval)

	// Collect records that need to be fetched
	for _, track := range tracks {
		if track == nil || myId == track.UserID || (track.StreamConditions == nil && track.DownloadConditions == nil) {
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
			if track.StreamConditions.TokenGate != nil {
				tokenGateTokenMints[track.StreamConditions.TokenGate.TokenMint] = struct{}{}
			}
			if track.StreamConditions.UsdcPurchase != nil {
				trackIDs[track.TrackID] = struct{}{}
				for _, playlistID := range track.PlaylistsContainingTrack {
					playlistIDs[playlistID] = struct{}{}
				}
				if removals := parseTrackRemovals(track.TrackID, track.PlaylistsPreviouslyContainingTrack); len(removals) > 0 {
					prevRemovals[track.TrackID] = removals
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
			if track.DownloadConditions.TokenGate != nil {
				tokenGateTokenMints[track.DownloadConditions.TokenGate.TokenMint] = struct{}{}
			}
			if track.DownloadConditions.UsdcPurchase != nil {
				trackIDs[track.TrackID] = struct{}{}
				for _, playlistID := range track.PlaylistsContainingTrack {
					playlistIDs[playlistID] = struct{}{}
				}
				if removals := parseTrackRemovals(track.TrackID, track.PlaylistsPreviouslyContainingTrack); len(removals) > 0 {
					prevRemovals[track.TrackID] = removals
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

	tokenGateTokenMintsSlice := make([]string, 0, len(tokenGateTokenMints))
	for tokenMint := range tokenGateTokenMints {
		tokenGateTokenMintsSlice = append(tokenGateTokenMintsSlice, tokenMint)
	}

	playlistIDsSlice := make([]int32, 0, len(playlistIDs))
	for id := range playlistIDs {
		playlistIDsSlice = append(playlistIDsSlice, id)
	}

	// Query for followed users
	followedUsers := make(map[int32]bool)
	tippedUsers := make(map[int32]bool)
	purchasedTracks := make(map[int32]bool)
	purchasedPlaylists := make(map[int32]bool)
	// tracks whose access survives via an album bought before the track left it
	prevPurchasedTracks := make(map[int32]bool)
	userTokenBalances := make(map[string]int64)
	walletTokenBalances := make(map[string]int64)
	coinDecimals := make(map[string]int32)

	g, ctx := errgroup.WithContext(ctx)

	// Query for followed users
	if len(followUserIDsSlice) > 0 {
		g.Go(func() error {
			rows, err := q.db.Query(ctx, `
				SELECT followee_user_id
				FROM follows
				WHERE follower_user_id = $1
				AND followee_user_id = ANY($2)
			`, myId, followUserIDsSlice)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var userID int32
				if err := rows.Scan(&userID); err == nil {
					followedUsers[userID] = true
				}
			}
			return rows.Err()
		})
	}

	// Query for tipped users
	if len(tipUserIDsSlice) > 0 {
		g.Go(func() error {
			rows, err := q.db.Query(ctx, `
				SELECT DISTINCT receiver_user_id
				FROM aggregate_user_tips
				WHERE sender_user_id = $1
				AND receiver_user_id = ANY($2)
				AND amount >= 0
			`, myId, tipUserIDsSlice)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var userID int32
				if err := rows.Scan(&userID); err == nil {
					tippedUsers[userID] = true
				}
			}
			return rows.Err()
		})
	}

	// Query for purchased tracks
	if len(trackIDsSlice) > 0 {
		g.Go(func() error {
			rows, err := q.db.Query(ctx, `
				SELECT content_id
				FROM v_usdc_purchases
				WHERE buyer_user_id = $1
				AND content_id = ANY($2)
				AND content_type = 'track'
			`, myId, trackIDsSlice)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var trackID int32
				if err := rows.Scan(&trackID); err == nil {
					purchasedTracks[trackID] = true
				}
			}
			return rows.Err()
		})
	}

	// Query for token balances
	if len(tokenGateTokenMintsSlice) > 0 {
		// Look up balances from the per-user aggregate table
		g.Go(func() error {
			rows, err := q.db.Query(ctx, `
				SELECT mint, COALESCE(balance, 0)
				FROM sol_user_balances
				WHERE user_id = $1
				AND mint = ANY($2)
			`, myId, tokenGateTokenMintsSlice)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var mint string
				var balance int64
				if err := rows.Scan(&mint, &balance); err == nil {
					userTokenBalances[mint] = balance
				}
			}
			return rows.Err()
		})

		// If a Solana wallet was provided (e.g. signed via middleware),
		// also check balances from the token account balances table.
		// Results are merged after g.Wait() to avoid concurrent map writes.
		if solanaWallet != "" {
			g.Go(func() error {
				rows, err := q.db.Query(ctx, `
					SELECT mint, COALESCE(balance, 0)
					FROM sol_token_account_balances
					WHERE owner = $1
					AND mint = ANY($2)
				`, solanaWallet, tokenGateTokenMintsSlice)
				if err != nil {
					return err
				}
				defer rows.Close()
				for rows.Next() {
					var mint string
					var balance int64
					if err := rows.Scan(&mint, &balance); err == nil {
						walletTokenBalances[mint] += balance
					}
				}
				return rows.Err()
			})
		}

		// Query for coin decimals
		g.Go(func() error {
			rows, err := q.db.Query(ctx, `
				SELECT mint, decimals
				FROM artist_coins
				WHERE mint = ANY($1)
			`, tokenGateTokenMintsSlice)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var mint string
				var decimals int32
				if err := rows.Scan(&mint, &decimals); err == nil {
					coinDecimals[mint] = decimals
				}
			}
			return rows.Err()
		})
	}

	// Query for purchased playlists
	if len(playlistIDsSlice) > 0 {
		g.Go(func() error {
			rows, err := q.db.Query(ctx, `
				SELECT content_id
				FROM v_usdc_purchases
				WHERE buyer_user_id = $1
				AND content_id = ANY($2)
				AND content_type = 'album'
			`, myId, playlistIDsSlice)
			if err != nil {
				return err
			}
			defer rows.Close()
			for rows.Next() {
				var playlistID int32
				if err := rows.Scan(&playlistID); err == nil {
					purchasedPlaylists[playlistID] = true
				}
			}
			return rows.Err()
		})
	}

	// Query for previously purchased playlists
	if len(prevRemovals) > 0 {
		// Flatten to (track, album, removal time) triples. Matching has to be
		// done on the pair: a purchase covers a track only if it predates that
		// track's removal from that album, not the album's earliest removal.
		flat := make([]trackRemoval, 0, len(prevRemovals))
		for _, removals := range prevRemovals {
			flat = append(flat, removals...)
		}

		if len(flat) > 0 {
			payload, err := json.Marshal(flat)
			if err != nil {
				return nil, err
			}
			g.Go(func() error {
				rows, err := q.db.Query(ctx, `
					SELECT DISTINCT r.track_id
					FROM jsonb_to_recordset($2::jsonb)
						AS r(track_id int, playlist_id int, removal_time bigint)
					JOIN v_usdc_purchases up
					  ON up.content_id = r.playlist_id
					 AND up.content_type = 'album'
					 AND up.buyer_user_id = $1
					WHERE up.created_at <= to_timestamp(r.removal_time)
				`, myId, payload)
				if err != nil {
					return err
				}
				defer rows.Close()
				for rows.Next() {
					var trackID int32
					if err := rows.Scan(&trackID); err == nil {
						prevPurchasedTracks[trackID] = true
					}
				}
				return rows.Err()
			})
		}
	}

	// Wait for all queries to complete
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Merge wallet balances by summing with user balances
	for mint, balance := range walletTokenBalances {
		userTokenBalances[mint] += balance
	}

	// Now determine access for each track
	for _, track := range tracks {
		if track == nil {
			continue
		}

		if myId == track.UserID {
			result[track.TrackID] = Access{
				Stream:   true,
				Download: true,
			}
			continue
		}

		if track.StreamConditions == nil && track.DownloadConditions == nil {
			result[track.TrackID] = Access{
				Stream:   true,
				Download: true,
			}
			continue
		}

		if track.StreamConditions != nil {
			hasAccess := false
			switch {
			case track.StreamConditions.FollowUserID != nil:
				hasAccess = followedUsers[int32(*track.StreamConditions.FollowUserID)]
			case track.StreamConditions.TipUserID != nil:
				hasAccess = tippedUsers[int32(*track.StreamConditions.TipUserID)]
			case track.StreamConditions.TokenGate != nil:
				tokenMint := track.StreamConditions.TokenGate.TokenMint
				requiredAmount := track.StreamConditions.TokenGate.TokenAmount
				if decimals, exists := coinDecimals[tokenMint]; exists {
					requiredAmount = requiredAmount * int64(math.Pow10(int(decimals)))
				}
				userBalance := userTokenBalances[tokenMint]
				hasAccess = userBalance >= requiredAmount
			case track.StreamConditions.UsdcPurchase != nil:
				// Check direct purchase
				hasAccess = purchasedTracks[track.TrackID]

				// Check current playlist purchases
				if !hasAccess {
					for _, playlistID := range track.PlaylistsContainingTrack {
						if purchasedPlaylists[playlistID] {
							hasAccess = true
							break
						}
					}
				}

				// Bought an album that used to contain this track, before it
				// left: access survives the removal.
				if !hasAccess {
					hasAccess = prevPurchasedTracks[track.TrackID]
				}
			}
			result[track.TrackID] = Access{
				Stream:   hasAccess,
				Download: hasAccess,
			}
			continue
		}

		// Check download access
		if track.DownloadConditions != nil {
			hasAccess := false
			switch {
			case track.DownloadConditions.FollowUserID != nil:
				hasAccess = followedUsers[int32(*track.DownloadConditions.FollowUserID)]
			case track.DownloadConditions.TipUserID != nil:
				hasAccess = tippedUsers[int32(*track.DownloadConditions.TipUserID)]
			case track.DownloadConditions.TokenGate != nil:
				tokenMint := track.DownloadConditions.TokenGate.TokenMint
				requiredAmount := track.DownloadConditions.TokenGate.TokenAmount
				if decimals, exists := coinDecimals[tokenMint]; exists {
					requiredAmount = requiredAmount * int64(math.Pow10(int(decimals)))
				}
				userBalance := userTokenBalances[tokenMint]
				hasAccess = userBalance >= requiredAmount
			case track.DownloadConditions.UsdcPurchase != nil:
				// Check direct purchase
				hasAccess = purchasedTracks[track.TrackID]

				// Check current playlist purchases
				if !hasAccess {
					for _, playlistID := range track.PlaylistsContainingTrack {
						if purchasedPlaylists[playlistID] {
							hasAccess = true
							break
						}
					}
				}

				// Bought an album that used to contain this track, before it
				// left: access survives the removal.
				if !hasAccess {
					hasAccess = prevPurchasedTracks[track.TrackID]
				}
			}
			// If there are download conditions, there is always stream access
			result[track.TrackID] = Access{
				Stream:   true,
				Download: hasAccess,
			}
			continue
		}
	}

	return result, nil
}
