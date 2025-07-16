package indexer

import (
	"strconv"
	"testing"
	"time"

	"bridgerton.audius.co/config"
	"bridgerton.audius.co/database"
	"bridgerton.audius.co/solana/spl/programs/payment_router"
	"github.com/gagliardetto/solana-go"
	"github.com/test-go/testify/assert"
	"github.com/test-go/testify/require"
)

func TestPurchaseValidation(t *testing.T) {

	ctx := t.Context()

	pool := database.CreateTestDatabase(t, "test_solana_indexer")

	sellerUserId := 1
	priceCents := 100
	priceUsdc := float64(priceCents * 10000)
	payoutWallet := solana.NewWallet().PublicKey()
	networkSplit := priceUsdc * config.Cfg.NetworkTakeRate / 100.0
	payoutSplit := priceUsdc - networkSplit

	database.Seed(pool, database.FixtureMap{
		"users": {
			{
				"user_id": sellerUserId,
			},
		},
		"user_payout_wallet_history": {
			{
				"user_id":                sellerUserId,
				"spl_usdc_payout_wallet": payoutWallet.String(),
			},
		},
		"track_price_history": {
			{
				"track_id":          1,
				"splits":            `[{"user_id": ` + strconv.Itoa(sellerUserId) + `, "percentage": 100}]`,
				"total_price_cents": priceCents,
			},
		},
	})

	{
		// valid purchase
		inst := payment_router.NewRouteInstruction(
			solana.PublicKey{},
			solana.PublicKey{},
			uint8(0),
			map[solana.PublicKey]uint64{
				payoutWallet: uint64(payoutSplit),
				config.Cfg.SolanaConfig.StakingBridgeUsdcTokenAccount: uint64(networkSplit),
			},
		)
		memo := parsedPurchaseMemo{
			ContentType:           "track",
			ContentId:             1,
			BuyerUserId:           2,
			ValidAfterBlocknumber: 100,
			AccessType:            "stream",
		}
		isValid, err := validatePurchase(
			ctx,
			config.Cfg,
			pool,
			inst,
			memo,
			time.Now().Add(time.Second))
		require.NoError(t, err)
		require.NotNil(t, isValid)
		assert.True(t, *isValid)
	}

	{
		// invalid purchase
		inst := payment_router.NewRouteInstruction(
			solana.PublicKey{},
			solana.PublicKey{},
			uint8(0),
			map[solana.PublicKey]uint64{
				payoutWallet: uint64(payoutSplit - 1),
				config.Cfg.SolanaConfig.StakingBridgeUsdcTokenAccount: uint64(networkSplit),
			},
		)
		memo := parsedPurchaseMemo{
			ContentType:           "track",
			ContentId:             1,
			BuyerUserId:           2,
			ValidAfterBlocknumber: 100,
			AccessType:            "stream",
		}
		isValid, err := validatePurchase(
			ctx,
			config.Cfg,
			pool,
			inst,
			memo,
			time.Now().Add(time.Second))
		require.Error(t, err)
		require.NotNil(t, isValid)
		assert.False(t, *isValid)
	}

	{
		// pending purchase
		inst := payment_router.NewRouteInstruction(
			solana.PublicKey{},
			solana.PublicKey{},
			uint8(0),
			map[solana.PublicKey]uint64{
				payoutWallet: uint64(payoutSplit - 1),
				config.Cfg.SolanaConfig.StakingBridgeUsdcTokenAccount: uint64(networkSplit),
			},
		)
		memo := parsedPurchaseMemo{
			ContentType:           "track",
			ContentId:             1,
			BuyerUserId:           2,
			ValidAfterBlocknumber: 102,
			AccessType:            "stream",
		}
		isValid, err := validatePurchase(
			ctx,
			config.Cfg,
			pool,
			inst,
			memo,
			time.Now().Add(time.Second))
		require.NoError(t, err)
		require.Nil(t, isValid)
	}
}
