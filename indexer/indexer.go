package indexer

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"api.audius.co/config"
	dbv1 "api.audius.co/database"
	"api.audius.co/jobs"
	"api.audius.co/logging"
	"connectrpc.com/connect"
	corev1connect "github.com/OpenAudio/go-openaudio/pkg/api/core/v1/v1connect"
	etl "github.com/OpenAudio/go-openaudio/pkg/etl"
	em "github.com/OpenAudio/go-openaudio/pkg/etl/processors/entity_manager"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// CoreIndexer runs the OpenAudio ETL indexer plus the dependent api/-side
// background jobs (aggregates, parity jobs, etc.). The block-fetching and
// entity-manager dispatch loop that previously lived here was a stub that
// only handled CreateUser — vendoring ETL via
// `github.com/OpenAudio/go-openaudio/pkg/etl` gives us the full 31-entity-type
// handler suite, materialized-view refresher, and scheduled-release publisher
// in one package, kept in sync with upstream releases.
type CoreIndexer struct {
	aggregatesCalculator *AggregatesCalculator
	etlIndexer           *etl.Indexer
	pool                 dbv1.DbPool
	openAudioSDK         *sdk.OpenAudioSDK
	Config               config.Config
	logger               *zap.Logger
}

func NewIndexer(cfg config.Config) *CoreIndexer {
	logger := logging.NewZapLogger(cfg).Named("CoreIndexer")

	connConfig, err := pgxpool.ParseConfig(cfg.WriteDbUrl)
	if err != nil {
		panic(fmt.Errorf("error parsing database URL: %w", err))
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), connConfig)
	if err != nil {
		panic(fmt.Errorf("error connecting to database: %w", err))
	}

	openAudioSDK := sdk.NewOpenAudioSDK(cfg.AudiusdURL)
	aggregatesCalculator := NewAggregatesCalculator(cfg)

	// ETL needs the Connect/gRPC Core client (for block fetching) and a DB URL.
	// SkipMigrations stays false (default), so bumping the pkg/etl module runs
	// whatever migrations it brought with it against api/'s database, on the
	// next indexer start. They track state in their own `etl_db_migrations`
	// table, separate from api/'s `schema_version`, and only up-migrations run
	// (RunDownMigrations is left false).
	//
	// None of them touches row data, which is the line being held: a module bump
	// reaches this database automatically, so anything that repairs data belongs
	// in ddl/, where it goes through review and the pre-roll migrate Job. 0035
	// is the worked example — it adds users_current_uniq_idx, while the delete
	// that makes that index creatable lives in ddl 0237.
	//
	// They are not, however, purely additive: 0026 drops a constraint and 0027
	// drops and recreates playlist_seen's primary key. A bump can alter the
	// schema here, so read what a version brings before taking it.
	//
	// The corollary is that an ETL migration can depend on a ddl one having
	// run. 0035 fails with a unique violation if ddl 0237 has not removed the
	// duplicates yet, which would stop the indexer starting. That ordering
	// holds because ddl migrations run in the pre-roll Job that every serving
	// Deployment depends on, and the ETL's run later, at indexer start.
	//
	// Two optional ETL components are disabled here because they don't fit
	// api/'s deployment:
	//   - MaterializedViewRefresh: refreshes mv_dashboard_* views that don't
	//     exist in api/'s schema (they were a go-openaudio-internal concern).
	//   - PgNotifyListener: publishes block/play events on a PG NOTIFY channel
	//     that api/ has no consumer for.
	// ScheduledReleasePublisher stays enabled — it's the same job apps' Python
	// `publish_scheduled_releases` celery task did and we want it running.
	etlCfg := etl.DefaultConfig()
	etlCfg.DisableMaterializedViewRefresh()
	etlCfg.DisablePgNotifyListener()
	etlCfg.ReadDataTypesEnv() // honors OPENAUDIO_ETL_ENTITY_MANAGER_DATA_TYPES if set
	etlCfg.BlockStreamEnabled = cfg.CoreBlockStreamEnabled

	etlIndexer := etl.New(openAudioSDK.Core, logger)
	etlIndexer.SetConfig(etlCfg)
	etlIndexer.SetDBURL(cfg.WriteDbUrl)
	etlIndexer.SetCheckReadiness(true)

	// Chain cutover bounds. Unset (0) is normal operation: resume from the last
	// indexed block, and never stop.
	//
	// The resume path is MAX(block_height) FROM etl_blocks, which carries no
	// chain_id. Pointed at a new chain it resolves to the old chain's height and
	// waits for a block that will not exist for years — a silent stall, not an
	// error. There is a chain-aware fallback to core_indexed_blocks, but it only
	// runs when etl_blocks is empty, so it never fires on a database that has
	// indexed the old chain. An explicit start height is the way across.
	if cfg.EtlStartingBlockHeight > 0 {
		etlIndexer.SetStartingBlockHeight(cfg.EtlStartingBlockHeight)
		logger.Warn("etl: starting from an explicit height, NOT resuming -- clear etlStartingBlockHeight once the cutover is done, or every restart re-indexes from here and duplicates plays",
			zap.Int64("height", cfg.EtlStartingBlockHeight))
	}
	if cfg.EtlEndingBlockHeight > 0 {
		etlIndexer.SetEndingBlockHeight(cfg.EtlEndingBlockHeight)
		logger.Warn("etl: will stop after this block and not resume -- clear etlEndingBlockHeight once the cutover is done",
			zap.Int64("height", cfg.EtlEndingBlockHeight))
	}

	// When enabled, source blocks from the CoreService.StreamBlocks gRPC stream
	// instead of polling GetBlocks. The ETL falls back to polling automatically
	// if the endpoint doesn't support it, so this is safe to flip on per-env.
	// Kept separate from the SDK's unary Core client (used for status + fallback).
	if cfg.CoreBlockStreamEnabled {
		etlIndexer.SetBlockStreamClient(newCoreStreamClient(cfg.AudiusdURL))
		logger.Info("etl block source: gRPC stream", zap.String("endpoint", cfg.AudiusdURL))
	}

	// Restore the pre-vendor setPubkeyForUser behavior via the upstream
	// post-create hook (go-openaudio #317). Recovers the EIP-712 pubkey
	// from each User Create tx and writes it to user_pubkeys in the same
	// DB transaction as the user row.
	etlIndexer.SetUserCreatedHook(newUserPubkeyHook(cfg, logger))

	// Index the user metadata `events` object (is_mobile_user, referrer)
	// into user_events on User Create + Update. This is the on-chain
	// source the mobile-install / referral challenge processors reconcile
	// from. Registered on both actions since the fields can arrive on a
	// create or a later profile update.
	userEventsHook := newUserEventsHook(logger)
	etlIndexer.SetUserCreatedHook(userEventsHook)
	etlIndexer.RegisterPostHook(em.EntityTypeUser, em.ActionUpdate, userEventsHook)

	// Write each on-chain play into the `plays` table, restoring the legacy
	// Python `index_core_plays` behavior. The vendored ETL play processor
	// only writes `etl_plays` (which nothing in api/ reads); this hook
	// bridges plays into the `plays` table every downstream consumer (the
	// on_play trigger's aggregates/milestones/notifications, the challenge
	// processors, trending, hourly-play-count) depends on. Runs in the same
	// DB tx as etl_plays, so the rows commit atomically.
	etlIndexer.RegisterPlaysHook(newPlaysHook(logger))

	return &CoreIndexer{
		aggregatesCalculator: aggregatesCalculator,
		etlIndexer:           etlIndexer,
		pool:                 pool,
		openAudioSDK:         openAudioSDK,
		Config:               cfg,
		logger:               logger,
	}
}

// newCoreStreamClient builds a gRPC (HTTP/2) CoreService client for the block
// stream. connect.WithGRPC requires HTTP/2, which the default transport
// negotiates over TLS. This is separate from the SDK's unary Core client, which
// the ETL still uses for status checks and the polling fallback.
func newCoreStreamClient(audiusdURL string) corev1connect.CoreServiceClient {
	baseURL := audiusdURL
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}
	return corev1connect.NewCoreServiceClient(http.DefaultClient, baseURL, connect.WithGRPC())
}

// Start runs the ETL indexer alongside the aggregates calculator. Both are
// long-lived; errgroup propagates the first error (and the ctx cancellation
// it triggers) to all members.
//
// Caveat: etl.Indexer.Run() uses its own internal context.Background() rather
// than honoring `ctx` — graceful shutdown via ctx cancellation isn't supported
// by the upstream API today. Process termination (SIGTERM) still works the
// way Go programs always do, and DB connections drain via pool finalizers on
// process exit. Acceptable tradeoff to avoid forking ETL.
func (ci *CoreIndexer) Start(ctx context.Context) error {
	eg := errgroup.Group{}
	eg.Go(func() error {
		return ci.aggregatesCalculator.Start(ctx)
	})
	eg.Go(func() error {
		ci.logger.Info("Starting ETL indexer")
		return ci.etlIndexer.Run()
	})
	ci.startParityJobs(ctx)
	return eg.Wait()
}

// startParityJobs schedules the periodic jobs that mirror what the legacy
// Python discovery-provider celery beat used to run. Each job's ScheduleEvery
// launches its own goroutine and exits when ctx is cancelled, so we don't
// need to add them to the errgroup — they self-manage.
//
// Intervals match apps' celery beat_schedule in src/app.py where applicable.
// update_delist_statuses isn't in apps' beat (apps invokes it externally),
// so we pick a conservative default.
func (ci *CoreIndexer) startParityJobs(ctx context.Context) {
	jobs.NewHourlyPlayCountsJob(ci.Config, ci.pool).
		ScheduleEvery(ctx, 30*time.Second)

	jobs.NewPrunePlaysJob(ci.Config, ci.pool).
		ScheduleEvery(ctx, 30*time.Second)

	jobs.NewUserListeningHistoryJob(ci.Config, ci.pool).
		ScheduleEvery(ctx, 5*time.Second)

	// Hourly to match discovery's effective cadence (trending_refresh_seconds
	// default 3600). The vendored port dropped that gate, so a 10s schedule ran
	// the multi-minute recompute continuously and IO-starved the block loop.
	jobs.NewTrendingJob(ci.Config, ci.pool).
		ScheduleEvery(ctx, 1*time.Hour)

	jobs.NewUpdateDelistStatusesJob(ci.Config, ci.pool).
		ScheduleEvery(ctx, 5*time.Minute)

	// Drift backstop: recompute aggregate counts from source tables to correct
	// any hot-path trigger delta divergence. 10m matches discovery's
	// update_aggregates celery cadence. Column-disjoint from the score-only
	// AggregatesCalculator, so they can run concurrently.
	jobs.NewReconcileAggregatesJob(ci.Config, ci.pool).
		ScheduleEvery(ctx, 10*time.Minute)

	// Reconcile derived challenge state from source tables. Per-challenge
	// scanners live in api/jobs/challenges/.
	jobs.NewIndexChallengesJob(ci.Config, ci.pool).
		ScheduleEvery(ctx, 30*time.Second)

	// Time-based notifications that the legacy Python beat produced. Unlike
	// the event-driven notifications (handled by DB triggers), these fire on
	// a timer because they depend on elapsed time, not an indexed entity.
	jobs.NewEngagementNotificationsJob(ci.Config, ci.pool).
		ScheduleEvery(ctx, 10*time.Minute)

	jobs.NewListenStreakReminderJob(ci.Config, ci.pool).
		ScheduleEvery(ctx, 10*time.Second)

	// Backfill missing track bpm / musical_key from content-node audio
	// analyses. Mirrors apps' repair_audio_analyses celery task, whose beat
	// schedule ran every 3 minutes. Needs the SDK for content-node discovery.
	jobs.NewRepairAudioAnalysesJob(ci.Config, ci.pool, ci.openAudioSDK).
		ScheduleEvery(ctx, 3*time.Minute)
}

func (ci *CoreIndexer) Close() {
	ci.aggregatesCalculator.Close()
	ci.pool.Close()
	ci.logger.Sync()
}
