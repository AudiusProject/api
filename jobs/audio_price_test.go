package jobs

import (
	"context"
	"testing"

	"api.audius.co/database"
	"api.audius.co/solana/spl/programs/meteora_damm_v2"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeDammV2Fetcher struct{ pool *meteora_damm_v2.Pool }

func (f fakeDammV2Fetcher) GetPool(_ context.Context, _ solana.PublicKey) (*meteora_damm_v2.Pool, error) {
	return f.pool, nil
}

func TestAudioPriceJobOnchain(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	ctx := context.Background()
	cfg := newTestConfig()
	audioMint := cfg.SolanaConfig.MintAudio.String()
	usdcMint := cfg.SolanaConfig.MintUSDC.String()

	// AUDIO stats row must exist (the job UPDATEs it). Start from a stale price.
	database.Seed(pool, database.FixtureMap{
		"artist_coin_stats": {{"mint": audioMint, "price": 999.0}},
	})

	// Real sqrt_price from the mainnet Meteora DAMM v2 AUDIO/USDC pool
	// (Ha6tnG7...), which decodes to AUDIO ≈ $0.012142 at 8/6 decimals.
	poolState := &meteora_damm_v2.Pool{
		TokenAMint: solana.MustPublicKeyFromBase58(audioMint),
		TokenBMint: solana.MustPublicKeyFromBase58(usdcMint),
		SqrtPrice:  bin.Uint128{Lo: 203268658239169394},
	}

	job := &AudioPriceJob{
		pool:      pool,
		fetcher:   fakeDammV2Fetcher{pool: poolState},
		audioMint: audioMint,
		usdcMint:  usdcMint,
		poolAddr:  solana.MustPublicKeyFromBase58("Ha6tnG7LrhsTyw4tyarQ59HxAKqpdbEc2yQZp9mrDM4h"),
		logger:    zap.NewNop(),
	}

	require.NoError(t, job.run(ctx))

	var price float64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT price FROM artist_coin_stats WHERE mint = $1`, audioMint).Scan(&price))
	assert.InDelta(t, 0.012142, price, 1e-5, "AUDIO/USD from pool sqrt_price via price_from_sqrt_price(_, 8, 6)")
}

func TestAudioPriceJobSkipsWithoutPool(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	// Zero pool address (as on dev) -> job no-ops without touching the DB.
	job := &AudioPriceJob{pool: pool, logger: zap.NewNop()}
	require.NoError(t, job.run(context.Background()))
}
