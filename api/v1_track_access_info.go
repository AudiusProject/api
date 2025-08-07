package api

import (
	"fmt"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
)

type ExtendedSplit struct {
	UserID       *int32  `json:"user_id,omitempty"`
	Percentage   float64 `json:"percentage"`
	PayoutWallet string  `json:"payout_wallet,omitempty"`
	EthWallet    *string `json:"eth_wallet,omitempty"`
	Amount       int64   `json:"amount"`
}

type ExtendedPurchaseGate struct {
	Price  *float64        `json:"price"`
	Splits []ExtendedSplit `json:"splits"`
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

func getExtendedPurchaseGate(gate *dbv1.FullAccessGate, userMap map[int32]dbv1.FullUser) *ExtendedAccessGate {
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

	// Handle USDC purchase gates
	price := gate.UsdcPurchase.Price
	originalSplits := gate.UsdcPurchase.Splits
	fmt.Println("originalSplits", originalSplits)

	extendedSplits := []ExtendedSplit{}
	for wallet, split := range originalSplits {
		userID := gate.UsdcPurchase.UserIds[wallet]
		extSplit := ExtendedSplit{
			Amount: split,
		}
		if user, exists := userMap[userID]; exists {
			extSplit.UserID = &userID
			extSplit.EthWallet = &user.Wallet.String
			extSplit.PayoutWallet = user.PayoutWallet
			// Percentage is amount / (total amount - network amount)
		} else if wallet == config.Cfg.SolanaConfig.StakingBridgeUsdcTokenAccount.String() {
			extSplit.PayoutWallet = wallet
			// Percentage is amount / total amount
		}
		extendedSplits = append(extendedSplits, extSplit)
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

	// Get all user IDs from the original splits to build user map
	userIDs := make(map[int32]struct{})
	if track.StreamConditions != nil && track.StreamConditions.UsdcPurchase != nil {
		for _, userId := range track.StreamConditions.UsdcPurchase.UserIds {
			userIDs[userId] = struct{}{}
		}
	}
	if track.DownloadConditions != nil && track.DownloadConditions.UsdcPurchase != nil {
		for _, userId := range track.DownloadConditions.UsdcPurchase.UserIds {
			userIDs[userId] = struct{}{}
		}
	}

	userIDSlice := make([]int32, 0, len(userIDs))
	for userID := range userIDs {
		userIDSlice = append(userIDSlice, userID)
	}

	// Fetch full users
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

	// Make extended access gates
	var extendedStreamConditions *ExtendedAccessGate
	if track.StreamConditions != nil {
		extendedStreamConditions = getExtendedPurchaseGate(track.StreamConditions, userMap)
	}
	var extendedDownloadConditions *ExtendedAccessGate
	if track.DownloadConditions != nil {
		extendedDownloadConditions = getExtendedPurchaseGate(track.DownloadConditions, userMap)
	}

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
