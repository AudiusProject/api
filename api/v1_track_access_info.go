package api

import (
	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"bridgerton.audius.co/utils"
	"github.com/gofiber/fiber/v2"
)

type ExtendedPurchaseGate struct {
	Price  *float64              `json:"price"`
	Splits []utils.ExtendedSplit `json:"splits"`
}

type ExtendedAccessGate struct {
	UsdcPurchase  *ExtendedPurchaseGate `json:"usdc_purchase,omitempty"`
	FollowUserID  *int64                `json:"follow_user_id,omitempty"`
	TipUserID     *int64                `json:"tip_user_id,omitempty"`
	NftCollection *map[string]any       `json:"nft_collection,omitempty"`
}

type TrackAccessInfoResponse struct {
	Access             dbv1.Access         `json:"access"`
	UserId             trashid.HashId      `json:"user_id"`
	Blocknumber        int32               `json:"blocknumber"`
	IsStreamGated      bool                `json:"is_stream_gated"`
	StreamConditions   *ExtendedAccessGate `json:"stream_conditions"`
	IsDownloadGated    bool                `json:"is_download_gated"`
	DownloadConditions *ExtendedAccessGate `json:"download_conditions"`
}

func getExtendedPurchaseGate(gate *dbv1.AccessGate, userMap map[int32]dbv1.FullUser) *ExtendedAccessGate {
	if gate == nil {
		return nil
	}

	// Handle non-purchase gates
	if gate.UsdcPurchase == nil {
		return &ExtendedAccessGate{
			FollowUserID:  gate.FollowUserID,
			TipUserID:     gate.TipUserID,
			NftCollection: gate.NftCollection,
		}
	}

	// Process USDC purchase gate
	price := gate.UsdcPurchase.Price
	originalSplits := gate.UsdcPurchase.Splits

	// Convert PurchaseSplit to utils.Split
	splits := make([]utils.Split, len(originalSplits))
	for i, split := range originalSplits {
		userID := int(split.UserID)
		splits[i] = utils.Split{
			UserID:     &userID,
			Percentage: split.Percentage,
		}
	}

	// Calculate extended splits with amounts using the utils function
	priceInCents := int(price) // Convert dollars to cents
	extendedSplits, err := utils.CalculateSplits(&priceInCents, splits, nil)
	if err != nil {
		// Handle error - return original structure without extended info
		return &ExtendedAccessGate{
			UsdcPurchase: &ExtendedPurchaseGate{
				Price:  &price,
				Splits: []utils.ExtendedSplit{},
			},
		}
	}

	// Add wallet information to extended splits
	for i := range extendedSplits {
		if extendedSplits[i].UserID != nil {
			userID := int32(*extendedSplits[i].UserID)
			if user, exists := userMap[userID]; exists {
				// Convert pgtype.Text to *string
				var ethWallet *string
				if user.Wallet.Valid {
					ethWallet = &user.Wallet.String
				}
				extendedSplits[i].EthWallet = ethWallet

				// Use PayoutWallet from user if available, otherwise use UsdcWallet
				if user.PayoutWallet != "" {
					extendedSplits[i].PayoutWallet = user.PayoutWallet
				} else if user.UsdcWallet.Valid {
					extendedSplits[i].PayoutWallet = user.UsdcWallet.String
				}
			}
		}
	}

	return &ExtendedAccessGate{
		UsdcPurchase: &ExtendedPurchaseGate{
			Price:  &price,
			Splits: extendedSplits,
		},
		FollowUserID:  gate.FollowUserID,
		TipUserID:     gate.TipUserID,
		NftCollection: gate.NftCollection,
	}
}

func (app *ApiServer) v1TrackAccessInfo(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	trackId := c.Locals("trackId").(int)

	// Get the track with extended information
	tracks, err := app.queries.FullTracks(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			MyID:            myId,
			Ids:             []int32{int32(trackId)},
			IncludeUnlisted: true,
		},
	})
	if err != nil {
		return err
	}

	if len(tracks) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "track not found")
	}

	track := tracks[0]

	// Get raw track data to access original splits
	rawTracks, err := app.queries.GetTracks(c.Context(), dbv1.GetTracksParams{
		MyID:            myId,
		Ids:             []int32{int32(trackId)},
		IncludeUnlisted: true,
	})
	if err != nil {
		return err
	}

	if len(rawTracks) == 0 {
		return fiber.NewError(fiber.StatusNotFound, "raw track not found")
	}

	rawTrack := rawTracks[0]

	// Get all user IDs from the original splits to build user map
	userIDs := make(map[int32]struct{})

	// Collect user IDs from stream conditions
	if rawTrack.StreamConditions != nil && rawTrack.StreamConditions.UsdcPurchase != nil {
		for _, split := range rawTrack.StreamConditions.UsdcPurchase.Splits {
			userIDs[split.UserID] = struct{}{}
		}
	}

	// Collect user IDs from download conditions
	if rawTrack.DownloadConditions != nil && rawTrack.DownloadConditions.UsdcPurchase != nil {
		for _, split := range rawTrack.DownloadConditions.UsdcPurchase.Splits {
			userIDs[split.UserID] = struct{}{}
		}
	}

	// Convert user IDs to slice
	userIDSlice := make([]int32, 0, len(userIDs))
	for userID := range userIDs {
		userIDSlice = append(userIDSlice, userID)
	}

	// Get user information for wallet data
	userMap := make(map[int32]dbv1.FullUser)
	if len(userIDSlice) > 0 {
		users, err := app.queries.FullUsers(c.Context(), dbv1.GetUsersParams{
			MyID: myId,
			Ids:  userIDSlice,
		})
		if err != nil {
			return err
		}

		for _, user := range users {
			userMap[user.UserID] = user
		}
	}

	// Process stream conditions with extended purchase gate information
	var extendedStreamConditions *ExtendedAccessGate
	if rawTrack.StreamConditions != nil {
		extendedStreamConditions = getExtendedPurchaseGate(rawTrack.StreamConditions, userMap)
	}

	// Process download conditions with extended purchase gate information
	var extendedDownloadConditions *ExtendedAccessGate
	if rawTrack.DownloadConditions != nil {
		extendedDownloadConditions = getExtendedPurchaseGate(rawTrack.DownloadConditions, userMap)
	}

	// Create response with extended access gate information
	response := TrackAccessInfoResponse{
		Access:             track.Access,
		UserId:             track.UserID,
		Blocknumber:        track.Blocknumber.Int32,
		IsStreamGated:      track.IsStreamGated.Bool,
		StreamConditions:   extendedStreamConditions,
		IsDownloadGated:    track.IsDownloadGated.Bool,
		DownloadConditions: extendedDownloadConditions,
	}

	return c.JSON(fiber.Map{
		"data": response,
	})
}
