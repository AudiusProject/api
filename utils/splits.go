package utils

import (
	"fmt"
	"sort"

	"bridgerton.audius.co/config"
)

type Split struct {
	UserID     *int    `json:"user_id"` // optional because staking bridge does not have user_id
	Percentage float64 `json:"percentage"`
}

type ExtendedSplit struct {
	UserID       *int    `json:"user_id"`
	Percentage   float64 `json:"percentage"`
	Amount       int     `json:"amount"`
	PayoutWallet string  `json:"payout_wallet"`
	EthWallet    *string `json:"eth_wallet"` // optional because staking bridge does not have eth_wallet
}

type splitCalculation struct {
	Split
	Amount           int
	PayoutWallet     string
	EthWallet        *string
	AmountFractional float64
	Index            int
}

const (
	// Allow up to 6 decimal points for split percentages
	PercentageDecimals   = 6
	PercentageMultiplier = 1000000 // 10^6

	// Cents to USDC multiplier
	CentsToUsdcMultiplier = 10000 // 10^4
)

// CalculateSplits deterministically calculates the USDC amounts to pay to each person,
// adjusting for rounding errors and ensuring the total matches the price.
// This mirrors the Python calculate_split_amounts function.
func CalculateSplits(price *int, splits []Split, networkTakeRate *float64) ([]ExtendedSplit, error) {
	if price == nil || *price == 0 || len(splits) == 0 {
		return []ExtendedSplit{}, nil
	}

	// Use default network take rate if not provided
	takeRate := config.Cfg.NetworkTakeRate
	if networkTakeRate != nil {
		takeRate = *networkTakeRate
	}

	priceInUsdc := CentsToUsdcMultiplier * (*price)
	runningTotal := 0
	newSplits := make([]splitCalculation, 0, len(splits))

	// Deduct network cut from the total price
	networkCut := int(float64(priceInUsdc) * takeRate / 100)
	priceInUsdc -= networkCut

	for index, split := range splits {
		// Multiply percentage to make it a whole number
		percentageWhole := int(split.Percentage * PercentageMultiplier)

		// Do safe integer math on the price
		amount := float64(percentageWhole * priceInUsdc)

		// Divide by the percentage multiplier afterward, and convert percent
		amount = amount / (PercentageMultiplier * 100)

		// Round towards zero, it'll round up later as necessary
		amountInUsdc := int(amount)

		newSplit := splitCalculation{
			Split:            split,
			Amount:           amountInUsdc,
			AmountFractional: amount - float64(amountInUsdc),
			Index:            index,
		}

		newSplits = append(newSplits, newSplit)
		runningTotal += amountInUsdc
	}

	// Resolve rounding errors by iteratively choosing the highest fractional
	// rounding errors to round up until the running total is correct
	sort.Slice(newSplits, func(i, j int) bool {
		if newSplits[i].AmountFractional != newSplits[j].AmountFractional {
			return newSplits[i].AmountFractional > newSplits[j].AmountFractional // descending
		}
		return newSplits[i].Amount < newSplits[j].Amount // ascending for tie-breaker
	})

	index := 0
	for runningTotal < priceInUsdc {
		newSplits[index].Amount++
		runningTotal++
		index = (index + 1) % len(newSplits)
	}

	if runningTotal != priceInUsdc {
		return nil, fmt.Errorf("bad splits math: expected %d but got %d", priceInUsdc, runningTotal)
	}

	// Sort back to original order
	sort.Slice(newSplits, func(i, j int) bool {
		return newSplits[i].Index < newSplits[j].Index
	})

	// Convert to ExtendedSplit and build result
	result := make([]ExtendedSplit, 0, len(newSplits)+1)
	for _, split := range newSplits {
		result = append(result, ExtendedSplit{
			UserID:       split.UserID,
			Percentage:   split.Percentage,
			Amount:       split.Amount,
			PayoutWallet: split.PayoutWallet,
			EthWallet:    split.EthWallet,
		})
	}

	// Append network cut
	stakingBridgeWallet := config.Cfg.SolanaConfig.StakingBridgeUsdcTokenAccount.String()
	result = append(result, ExtendedSplit{
		UserID:       nil,
		Percentage:   takeRate,
		Amount:       networkCut,
		PayoutWallet: stakingBridgeWallet,
		EthWallet:    nil,
	})

	return result, nil
}
