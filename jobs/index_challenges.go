package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/jobs/challenges"
	"api.audius.co/logging"
	"go.uber.org/zap"
)

// IndexChallengesJob runs all registered challenge processors on a tick.
// Each processor runs inside its own pgx transaction; failures in one
// don't stop the others.
//
// Mirrors apps' index_challenges celery task in role but not implementation:
// where apps drains a Redis queue of dispatched events, we reconcile derived
// state from source tables. See package docs in jobs/challenges/processor.go
// for the rationale.
type IndexChallengesJob struct {
	pool       database.DbPool
	logger     *zap.Logger
	processors []challenges.Processor

	mutex     sync.Mutex
	isRunning bool
}

// NewIndexChallengesJob constructs the umbrella job with Phase 1 processors
// pre-wired.
func NewIndexChallengesJob(cfg config.Config, pool database.DbPool) *IndexChallengesJob {
	return &IndexChallengesJob{
		pool:   pool,
		logger: logging.NewZapLogger(cfg).Named("IndexChallengesJob"),
		processors: []challenges.Processor{
			// Phase 1
			&challenges.TrackUploadProcessor{},
			&challenges.FirstPlaylistProcessor{},
			&challenges.ProfileCompletionProcessor{},
			&challenges.ProfileVerifiedProcessor{},
			&challenges.ListenStreakProcessor{},
			challenges.NewPlayCount250Processor(),
			challenges.NewPlayCount1000Processor(),
			challenges.NewPlayCount10000Processor(),
			challenges.NewTrendingTrackProcessor(),
			challenges.NewTrendingUndergroundProcessor(),
			challenges.NewTrendingPlaylistProcessor(),
			// Phase 2
			&challenges.FirstWeeklyCommentProcessor{},
			&challenges.CommentPinProcessor{},
			&challenges.CosignProcessor{},
			&challenges.TastemakerProcessor{},
			&challenges.RemixContestWinnerProcessor{},
			challenges.NewAudioMatchingBuyerProcessor(),
			challenges.NewAudioMatchingSellerProcessor(),
			// Phase 3 (signal-driven)
			&challenges.MobileInstallProcessor{},
			&challenges.OneShotProcessor{},
			challenges.NewReferralProcessor(),
			challenges.NewVerifiedReferralProcessor(),
			&challenges.ReferredProcessor{},
		},
	}
}

// ScheduleEvery runs the job every `interval` until the context is cancelled.
func (j *IndexChallengesJob) ScheduleEvery(ctx context.Context, interval time.Duration) *IndexChallengesJob {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				j.Run(ctx)
			case <-ctx.Done():
				j.logger.Info("Job shutting down")
				return
			}
		}
	}()
	return j
}

// Run executes the job once.
func (j *IndexChallengesJob) Run(ctx context.Context) {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	}
}

func (j *IndexChallengesJob) run(ctx context.Context) error {
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

	start := time.Now()
	var anyErr error
	for _, p := range j.processors {
		if err := j.runProcessor(ctx, p); err != nil {
			j.logger.Error("processor failed",
				zap.String("challenge_id", p.ChallengeID()),
				zap.Error(err))
			anyErr = err
			// Continue — one bad processor shouldn't kill the rest.
		}
	}
	j.logger.Info("Reconciled challenges",
		zap.Int("processors", len(j.processors)),
		zap.Duration("duration", time.Since(start)))
	return anyErr
}

// runProcessor runs a single processor in its own transaction.
func (j *IndexChallengesJob) runProcessor(ctx context.Context, p challenges.Processor) error {
	tx, err := j.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := p.Reconcile(ctx, tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
