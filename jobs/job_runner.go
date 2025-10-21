package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/logging"
	"go.uber.org/zap"
)

type JobRunner interface {
	Execute(ctx context.Context, logger *zap.Logger) error
}

type BaseJob struct {
	logger *zap.Logger
	runner JobRunner

	mutex     sync.Mutex
	isRunning bool
}

type BaseJobConfig struct {
	config  config.Config
	jobName string
	runner  JobRunner
}

func NewBaseJob(cfg BaseJobConfig) *BaseJob {
	logger := logging.NewZapLogger(cfg.config).Named(cfg.jobName)
	return &BaseJob{
		logger: logger,
		runner: cfg.runner,
	}
}

// Run executes the job once
func (j *BaseJob) Run(ctx context.Context) error {
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
		return err
	}
	j.logger.Info("Job completed successfully")
	return nil
}

func (j *BaseJob) run(ctx context.Context) error {
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

	return j.runner.Execute(ctx, j.logger)
}

// ScheduleEvery runs the job every `duration` until the context is cancelled.
func (j *BaseJob) ScheduleEvery(ctx context.Context, duration time.Duration) *BaseJob {
	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				j.Run(ctx)
			case <-ctx.Done():
				return
			}
		}
	}()
	return j
}
