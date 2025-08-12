package dbv1

import (
	"sort"

	"bridgerton.audius.co/config"
)

// struct for stream_conditions + download_conditions
type AccessGate struct {
	UsdcPurchase *PurchaseGate `json:"usdc_purchase,omitempty"`

	FollowUserID *int64 `json:"follow_user_id,omitempty"`

	TipUserID *int64 `json:"tip_user_id,omitempty"`

	NftCollection *map[string]any `json:"nft_collection,omitempty"`
}

type PurchaseSplit struct {
	UserID     int32   `json:"user_id"`
	Percentage float64 `json:"percentage"`
}

type PurchaseGate struct {
	Price  float64         `json:"price"`
	Splits []PurchaseSplit `json:"splits"`
}

func (gate PurchaseGate) ToFullPurchaseGate(cfg config.Config, userMap map[int32]FullUser) *FullPurchaseGate {
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
	networkWallet := cfg.SolanaConfig.StakingBridgeUsdcTokenAccount.String()
	splitMap[networkWallet] = int64(networkCut)

	splits := make([]FullSplit, 0, len(splitMap))
	for wallet, amount := range splitMap {
		split := FullSplit{
			Amount:       amount,
			PayoutWallet: wallet,
		}

		if wallet != networkWallet {
			// For user splits, calculate percentage based on the user portion (excluding network cut)
			split.Percentage = (float64(amount) / float64(price)) * 100.0

			for _, originalSplit := range gate.Splits {
				if user, exists := userMap[originalSplit.UserID]; exists && user.PayoutWallet == wallet {
					split.UserID = &originalSplit.UserID
					if user.Wallet.Valid {
						split.EthWallet = &user.Wallet.String
					}
					break
				}
			}
		} else {
			// For network wallet, calculate percentage based on total price
			split.Percentage = (float64(amount) / float64(priceInUsdc)) * 100.0
		}

		splits = append(splits, split)
	}

	// Sort splits by the payout_wallet
	sort.Slice(splits, func(i, j int) bool {
		return splits[i].PayoutWallet < splits[j].PayoutWallet
	})

	return &FullPurchaseGate{
		Price:  gate.Price,
		Splits: splits,
	}
}

type FullSplit struct {
	UserID       *int32  `json:"user_id,omitempty"`
	Percentage   float64 `json:"percentage"`
	PayoutWallet string  `json:"payout_wallet,omitempty"`
	EthWallet    *string `json:"eth_wallet,omitempty"`
	Amount       int64   `json:"amount"`
}

type FullPurchaseGate struct {
	Price  float64     `json:"price"`
	Splits []FullSplit `json:"splits"`
}
