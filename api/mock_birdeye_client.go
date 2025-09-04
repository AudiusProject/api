package api

import (
	"context"
	"fmt"

	"bridgerton.audius.co/birdeye"
)

type mockBirdeyeClient struct{}

func (m *mockBirdeyeClient) GetTokenOverview(ctx context.Context, mint string, frames string) (*birdeye.TokenOverview, error) {
	switch mint {
	case "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v":
		return &birdeye.TokenOverview{
			Address: "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v",
			Price:   1.0,
		}, nil
	case "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM":
		return &birdeye.TokenOverview{
			Address: "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
			Price:   10.0,
		}, nil
	}
	return nil, fmt.Errorf("token not found")
}

func (m *mockBirdeyeClient) GetPrices(ctx context.Context, mints []string) (birdeye.TokenPriceMap, error) {
	prices := make(birdeye.TokenPriceMap)
	for _, mint := range mints {
		switch mint {
		case "EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v":
			prices[mint] = birdeye.TokenPriceData{
				Value: 1.0,
			}
		case "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM":
			prices[mint] = birdeye.TokenPriceData{
				Value: 10.0,
			}
		default:
			return nil, fmt.Errorf("price not found for mint %s", mint)
		}
	}
	return prices, nil
}
