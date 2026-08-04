package jobs

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"api.audius.co/solana/spl/programs/meteora_damm_v2"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"go.uber.org/zap"
)

// dammV2PoolFetcher reads and decodes a Meteora DAMM v2 pool account.
// Abstracted so tests can inject a fake without a live RPC.
type dammV2PoolFetcher interface {
	GetPool(ctx context.Context, addr solana.PublicKey) (*meteora_damm_v2.Pool, error)
}

type rpcDammV2Fetcher struct{ rpc *rpc.Client }

func (f rpcDammV2Fetcher) GetPool(ctx context.Context, addr solana.PublicKey) (*meteora_damm_v2.Pool, error) {
	res, err := f.rpc.GetAccountInfo(ctx, addr)
	if err != nil {
		return nil, err
	}
	if res == nil || res.Value == nil || res.Value.Data == nil {
		return nil, fmt.Errorf("pool account %s not found", addr)
	}
	data := res.Value.Data.GetBinary()
	if len(data) < 8 || !bytes.Equal(data[:8], meteora_damm_v2.POOL_DISCRIMINATOR) {
		return nil, fmt.Errorf("account %s is not a DAMM v2 pool", addr)
	}
	var pool meteora_damm_v2.Pool
	if err := bin.NewBorshDecoder(data).Decode(&pool); err != nil {
		return nil, fmt.Errorf("failed to decode DAMM v2 pool %s: %w", addr, err)
	}
	return &pool, nil
}

// AudioPriceJob maintains the AUDIO USD anchor (artist_coin_stats.price for the
// AUDIO mint) that the artist_coin_prices view uses to convert every coin's
// AUDIO-denominated pool price to USD. It reads the on-chain Meteora DAMM v2
// AUDIO/USDC pool rather than Birdeye.
type AudioPriceJob struct {
	pool      database.DbPool
	fetcher   dammV2PoolFetcher
	audioMint string
	usdcMint  string
	poolAddr  solana.PublicKey
	logger    *zap.Logger

	mutex     sync.Mutex
	isRunning bool
}

func NewAudioPriceJob(config config.Config, pool database.DbPool) *AudioPriceJob {
	logger := logging.NewZapLogger(config).Named("AudioPriceJob")

	var fetcher dammV2PoolFetcher
	if len(config.SolanaConfig.RpcProviders) > 0 {
		fetcher = rpcDammV2Fetcher{rpc: rpc.New(config.SolanaConfig.RpcProviders[0])}
	}

	return &AudioPriceJob{
		pool:      pool,
		fetcher:   fetcher,
		audioMint: config.SolanaConfig.MintAudio.String(),
		usdcMint:  config.SolanaConfig.MintUSDC.String(),
		poolAddr:  config.SolanaConfig.AudioUsdcPool,
		logger:    logger,
	}
}

// ScheduleEvery runs the job every `duration` until the context is cancelled.
func (j *AudioPriceJob) ScheduleEvery(ctx context.Context, duration time.Duration) *AudioPriceJob {
	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				j.Run(ctx)
			case <-ctx.Done():
				j.logger.Info("Job schedule shutting down")
				return
			}
		}
	}()
	return j
}

// Run executes the job once
func (j *AudioPriceJob) Run(ctx context.Context) {
	j.logger.Info("Job started")
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	} else {
		j.logger.Info("Job completed successfully")
	}
}

// Reads the AUDIO/USDC DAMM v2 pool on-chain, derives AUDIO's USD price, and
// updates artist_coin_stats. Ensures only one instance runs at a time.
func (j *AudioPriceJob) run(ctx context.Context) error {
	j.mutex.Lock()
	if j.isRunning {
		j.mutex.Unlock()
		return fmt.Errorf("job is already running")
	}
	j.isRunning = true
	j.mutex.Unlock()
	defer func() {
		j.mutex.Lock()
		j.isRunning = false
		j.mutex.Unlock()
	}()

	// No AUDIO/USDC pool on dev — nothing to anchor from.
	if j.poolAddr.IsZero() {
		j.logger.Debug("no AUDIO/USDC pool configured; skipping AUDIO price update")
		return nil
	}
	if j.fetcher == nil {
		return fmt.Errorf("no RPC client configured")
	}

	poolState, err := j.fetcher.GetPool(ctx, j.poolAddr)
	if err != nil {
		return fmt.Errorf("failed to fetch AUDIO/USDC pool: %w", err)
	}

	// Guard the orientation this job assumes: token A = AUDIO, token B = USDC, so
	// price_from_sqrt_price returns token-B-per-token-A = USDC-per-AUDIO = AUDIO/USD.
	if poolState.TokenAMint.String() != j.audioMint || poolState.TokenBMint.String() != j.usdcMint {
		return fmt.Errorf("unexpected AUDIO/USDC pool ordering: tokenA=%s tokenB=%s",
			poolState.TokenAMint, poolState.TokenBMint)
	}

	sqrtPrice := poolState.SqrtPrice.BigInt()

	// AUDIO USD = price_from_sqrt_price(sqrt, 8 [AUDIO decimals], 6 [USDC decimals])
	// (USDC ≈ $1). Reuses the same SQL function the artist_coin_prices view uses.
	_, err = j.pool.Exec(ctx, `
		UPDATE artist_coin_stats
		SET price = price_from_sqrt_price($1::numeric, 8, 6),
		    updated_at = NOW()
		WHERE mint = $2
	`, sqrtPrice.String(), j.audioMint)
	if err != nil {
		return fmt.Errorf("failed to update AUDIO price: %w", err)
	}

	j.logger.Debug("Updated AUDIO price on-chain", zap.String("pool", j.poolAddr.String()))
	return nil
}
