package main

import (
	"context"
	"fmt"
	"os"

	"api.audius.co/config"
	"api.audius.co/jobs"
	"github.com/jackc/pgx/v5/pgxpool"
)

// remix-contest-notifications runs RemixContestNotificationsJob once and exits.
// Deploy as a Kubernetes CronJob on a 15-20 minute schedule; the job is
// idempotent via ON CONFLICT + NOT EXISTS guards so overlapping runs are safe.
//
// Required env vars (same as the main bridge):
//   writeDbUrl - postgres connection string
//   ENV        - "dev", "stage", or "prod"
func main() {
	cfg := config.Cfg
	ctx := context.Background()

	pool, err := pgxpool.New(ctx, cfg.WriteDbUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "remix-contest-notifications: db connect failed: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	job := jobs.NewRemixContestNotificationsJob(cfg, pool)
	if err := job.RunE(ctx); err != nil {
		// Individual step errors are already logged by the job via zap;
		// a non-zero exit tells the CronJob controller to record a failure.
		os.Exit(1)
	}
}
