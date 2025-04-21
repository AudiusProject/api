package dbv1

import (
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
	remainderMap := map[string]float64{}
	var sum int64
	for _, split := range gate.Splits {

		user := userMap[split.UserID]
		if user.PayoutWallet != "" {
			amountF64 := price * (split.Percentage / 100)
			amount := int64(amountF64)
			splitMap[user.PayoutWallet] = amount
			sum += amount
			remainderMap[user.PayoutWallet] = amountF64 - float64(amount)
		} else {
			// if user is not in map, or payout_wallet no exist
			// the rounding error will be split amongst other parties.
			// this should not happen, so we log it out at least?
			// todo: need a global logger of some sort here...
		}
	}

	// distribute rounding error across splits
	// using the wallet with the highest remainder first
	for sum < int64(price) {
		wallet := ""
		highest := 0.0
		for w, rem := range remainderMap {
			if rem > highest {
				highest = rem
				wallet = w
			}
		}
		splitMap[wallet] += 1
		remainderMap[wallet] -= 1
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
