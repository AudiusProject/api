package jobs

import (
	"context"
	"fmt"
	"strconv"
	"testing"
	"time"

	"api.audius.co/database"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// fakeSupplyFetcher returns canned SPL mint supplies (human units) keyed by mint.
type fakeSupplyFetcher struct {
	supplies map[string]float64
}

func (f fakeSupplyFetcher) GetTokenSupply(
	_ context.Context,
	mint solana.PublicKey,
	_ rpc.CommitmentType,
) (*rpc.GetTokenSupplyResult, error) {
	v, ok := f.supplies[mint.String()]
	if !ok {
		return nil, fmt.Errorf("no supply for %s", mint)
	}
	return &rpc.GetTokenSupplyResult{
		Value: &rpc.UiTokenAmount{
			Amount:         "0",
			Decimals:       6,
			UiAmountString: strconv.FormatFloat(v, 'f', -1, 64),
		},
	}, nil
}

func TestCoinStatsOnchainJob(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	ctx := context.Background()
	cfg := newTestConfig()
	audioMint := cfg.SolanaConfig.MintAudio.String()

	// A valid base58 SPL mint for the artist coin under test (GetTokenSupply parses it).
	const coinMint = "So11111111111111111111111111111111111111112"
	const quoteVault = "QVau1t1111111111111111111111111111111111111"

	// ~25h-old snapshot timestamp, within the job's [now-30h, now-24h] lookup window.
	snapshot24hAgo := time.Now().Add(-25 * time.Hour)

	fixtures := database.FixtureMap{
		"users": {
			{"user_id": 1, "wallet": "0x1234567890123456789012345678901234567890"},
		},
		"artist_coins": {
			{"ticker": "COIN1", "decimals": 6, "user_id": 1, "mint": coinMint},
			{"ticker": "AUDIO", "decimals": 8, "user_id": 1, "mint": audioMint},
		},
		// AUDIO USD anchor (normally maintained by AudioPriceJob).
		"artist_coin_stats": {
			{"mint": audioMint, "price": 2.0},
		},
		// On-chain USD price for the coin comes via artist_coin_prices -> pools_price_usd.
		"artist_coin_pools": {
			{"address": "pool1", "base_mint": coinMint, "price_usd": 4.0},
		},
		// Holders: 3 distinct owners with balance > 0, one with 0 (excluded).
		"sol_token_account_balances": {
			{"account": "acct1", "mint": coinMint, "owner": "owner1", "balance": 10, "slot": 1},
			{"account": "acct2", "mint": coinMint, "owner": "owner2", "balance": 5, "slot": 1},
			{"account": "acct3", "mint": coinMint, "owner": "owner3", "balance": 1, "slot": 1},
			{"account": "acct4", "mint": coinMint, "owner": "owner4", "balance": 0, "slot": 1},
		},
		// Volume through the DBC quote vault (AUDIO leg): |1e8| + |-2e8| = 3e8 -> 3 AUDIO.
		"sol_token_account_balance_changes": {
			{"signature": "sig1", "mint": audioMint, "owner": "trader1", "account": quoteVault,
				"change": 100_000_000, "balance": 100_000_000, "slot": 10, "block_timestamp": "2024-01-01 00:00:00"},
			{"signature": "sig2", "mint": audioMint, "owner": "trader2", "account": quoteVault,
				"change": -200_000_000, "balance": 0, "slot": 11, "block_timestamp": "2024-01-01 00:01:00"},
		},
		// Active DBC pool -> liquidity = base_reserve*price + quote_reserve*audioPrice
		// = (1e9/1e6)*4.0 + (5e10/1e8)*2.0 = 1000*4 + 500*2 = 5000.
		"sol_meteora_dbc_pools": {
			{"account": "dbcpool1", "base_mint": coinMint, "quote_vault": quoteVault,
				"base_reserve": 1_000_000_000, "quote_reserve": 50_000_000_000, "is_migrated": 0},
		},
		// Pre-existing on-chain AUDIO row to verify its price is not clobbered.
		"artist_coin_stats_onchain": {
			{"mint": audioMint, "price": 2.0},
		},
		// A ~25h-old price snapshot so the 24h change is computable: (4.0-2.0)/2.0*100 = 100%.
		"artist_coin_price_history": {
			{"mint": coinMint, "timestamp": snapshot24hAgo, "price": 2.0},
		},
	}
	database.Seed(pool, fixtures)

	job := &CoinStatsOnchainJob{
		pool:      pool,
		rpcClient: fakeSupplyFetcher{supplies: map[string]float64{coinMint: 1000, audioMint: 1_000_000}},
		audioMint: audioMint,
		logger:    zap.NewNop(),
	}

	require.NoError(t, job.run(ctx))

	// Assert the coin's derived stats.
	var (
		price         float64
		holder        int
		liquidity     float64
		totalSupply   float64
		marketCap     float64
		totalVolume   float64
		totalVolUSD   float64
		history24h    float64
		priceChange24 float64
	)
	err := pool.QueryRow(ctx, `
		SELECT price, holder, liquidity, total_supply, market_cap,
		       total_volume, total_volume_usd, history_24h_price, price_change_24h_percent
		FROM artist_coin_stats_onchain WHERE mint = $1`, coinMint).
		Scan(&price, &holder, &liquidity, &totalSupply, &marketCap,
			&totalVolume, &totalVolUSD, &history24h, &priceChange24)
	require.NoError(t, err)

	assert.InDelta(t, 4.0, price, 1e-9, "price from pools_price_usd")
	assert.Equal(t, 3, holder, "distinct owners with balance > 0")
	assert.InDelta(t, 5000.0, liquidity, 1e-6, "TVL = base_usd + quote_usd")
	assert.InDelta(t, 1000.0, totalSupply, 1e-9, "supply from RPC")
	assert.InDelta(t, 4000.0, marketCap, 1e-6, "price * supply")
	assert.InDelta(t, 3.0, totalVolume, 1e-9, "AUDIO volume through vault")
	assert.InDelta(t, 6.0, totalVolUSD, 1e-9, "volume * audioPrice")
	assert.InDelta(t, 2.0, history24h, 1e-9, "24h-ago snapshot")
	assert.InDelta(t, 100.0, priceChange24, 1e-6, "24h percent change")

	// AUDIO's maintained price must not be clobbered (on-chain price is NULL for it).
	var audioPrice float64
	err = pool.QueryRow(ctx,
		`SELECT price FROM artist_coin_stats_onchain WHERE mint = $1`, audioMint).Scan(&audioPrice)
	require.NoError(t, err)
	assert.InDelta(t, 2.0, audioPrice, 1e-9, "AUDIO price preserved")
}
