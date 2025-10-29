package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"api.audius.co/birdeye"
	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"go.uber.org/zap"
)

type AudioPriceJob struct {
	birdeyeClient *birdeye.Client
	pool          database.DbPool
	logger        *zap.Logger

	mutex     sync.Mutex
	isRunning bool
}

func NewAudioPriceJob(config config.Config, pool database.DbPool) *AudioPriceJob {
	logger := logging.NewZapLogger(config).Named("AudioPriceJob")
	birdeyeClient := birdeye.New(config.BirdeyeToken)

	return &AudioPriceJob{
		birdeyeClient: birdeyeClient,
		logger:        logger,
		pool:          pool,
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

// Gets the price for AUDIO from Birdeye and updates artist_coin_stats table.
// Ensures only one instance runs at a time.
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

	audioMint := "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM" // AUDIO mint
	priceData, err := j.birdeyeClient.GetPrice(ctx, audioMint)
	if err != nil {
		return fmt.Errorf("failed to get prices: %w", err)
	}

	_, err = j.pool.Exec(ctx, `
		UPDATE artist_coin_stats
		SET price = $1,
		    updated_at = NOW()
		WHERE mint = $2
	`, priceData.Value, audioMint)
	if err != nil {
		return fmt.Errorf("failed to update artist coin prices: %w", err)
	}

	j.logger.Debug("Updated AUDIO price",
		zap.Float64("price", priceData.Value),
	)

	return nil
}
