package dbv1

import (
	"math"

	"bridgerton.audius.co/config"
)

// struct for stream_conditions + download_conditions
type AccessGate struct {
	UsdcPurchase *PurchaseGate `json:"usdc_purchase,omitempty"`

	FollowUserID *int64 `json:"follow_user_id,omitempty"`

	TipUserID *int64 `json:"tip_user_id,omitempty"`

	NftCollection *map[string]any `json:"nft_collection,omitempty"`
}

type PurchaseGate struct {
	Price  float64 `json:"price"`
	Splits []struct {
		UserID     int32   `json:"user_id"`
		Percentage float64 `json:"percentage"`
	} `json:"splits"`
}

func (gate *PurchaseGate) toFullPurchaseGate(cfg config.Config, userMap map[int32]FullUser) *FullPurchaseGate {
	priceInUsdc := gate.Price * 10000
	networkCut := priceInUsdc * (cfg.NetworkTakeRate / 100)
	price := priceInUsdc - networkCut

	splitMap := map[string]int64{}
	var sum int64
	for _, split := range gate.Splits {
		// todo: if user is not in map, or payout_wallet no exist
		// the rounding error will be split amongst other parties, which seems fine.
		user := userMap[split.UserID]
		if user.PayoutWallet != "" {
			amount := int64(price * (split.Percentage / 100))
			splitMap[user.PayoutWallet] = amount
			sum += amount
		}
	}

	// distribute rounding error across splits
	// using the lowest value first
	for sum < int64(price) {
		// find lowest value
		wallet := ""
		lowest := int64(math.MaxInt64)
		for w, amount := range splitMap {
			if amount < lowest {
				lowest = amount
				wallet = w
			}
		}
		splitMap[wallet] += 1
		sum += 1
	}

	// add network take last (after rounding error is distributed)
	splitMap[cfg.StakingBridgeUsdcPayoutWallet] = int64(networkCut)
	return &FullPurchaseGate{
		Price:  gate.Price,
		Splits: splitMap,
	}
}

func (usage *AccessGate) toFullAccessGate(cfg config.Config, userMap map[int32]FullUser) *FullAccessGate {
	if usage == nil {
		return nil
	}
	if usage.UsdcPurchase != nil {
		return &FullAccessGate{
			UsdcPurchase: usage.UsdcPurchase.toFullPurchaseGate(cfg, userMap),
		}
	}
	return &FullAccessGate{
		AccessGate: *usage,
	}
}

type FullAccessGate struct {
	AccessGate
	UsdcPurchase *FullPurchaseGate `json:"usdc_purchase,omitempty"`
}

type FullPurchaseGate struct {
	Price  float64          `json:"price"`
	Splits map[string]int64 `json:"splits"`
}
