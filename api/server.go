package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/api/spl"
	"bridgerton.audius.co/api/spl/programs/reward_manager"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/trashid"
	"github.com/AudiusProject/audiusd/pkg/rewards"
	adapter "github.com/axiomhq/axiom-go/adapters/zap"
	"github.com/axiomhq/axiom-go/axiom"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gofiber/contrib/fiberzap/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	pgxzap "github.com/jackc/pgx-zap"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/maypok86/otter"
	"github.com/segmentio/encoding/json"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func InitLogger(config config.Config) *zap.Logger {
	// stdout core
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	consoleEncoder := zapcore.NewJSONEncoder(encoderConfig)
	stdoutCore := zapcore.NewCore(
		consoleEncoder,
		zapcore.AddSync(os.Stdout),
		zapcore.InfoLevel,
	)

	var core zapcore.Core = stdoutCore

	// axiom core, if token and dataset are provided
	if config.AxiomToken != "" && config.AxiomDataset != "" {
		axiomAdapter, err := adapter.New(
			adapter.SetClientOptions(
				axiom.SetAPITokenConfig(config.AxiomToken),
				axiom.SetOrganizationID("audius-Lu52"),
			),
			adapter.SetDataset(config.AxiomDataset),
		)
		if err != nil {
			panic(err)
		}

		core = zapcore.NewTee(stdoutCore, axiomAdapter)
	}

	logger := zap.New(core)
	return logger
}

func RequestTimer() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("start", time.Now())
		return c.Next()
	}
}

func NewApiServer(config config.Config) *ApiServer {
	logger := InitLogger(config)

	connConfig, err := pgxpool.ParseConfig(config.DbUrl)
	if err != nil {
		logger.Error("db connect failed", zap.Error(err))
	}

	// disable sql logging in ENV "test"
	if config.Env != "test" {
		connConfig.ConnConfig.Tracer = &tracelog.TraceLog{
			Logger:   pgxzap.NewLogger(logger),
			LogLevel: tracelog.LogLevelInfo,
		}
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), connConfig)

	if err != nil {
		logger.Fatal("db connect failed", zap.Error(err))
	}

	resolveHandleCache, err := otter.MustBuilder[string, int32](50_000).
		CollectStats().
		Build()
	if err != nil {
		panic(err)
	}

	resolveGrantCache, err := otter.MustBuilder[string, bool](50_000).
		WithTTL(60 * time.Minute).
		CollectStats().
		Build()
	if err != nil {
		panic(err)
	}

	privateKey, err := crypto.HexToECDSA(config.DelegatePrivateKey)
	if err != nil {
		panic(err)
	}

	solanaRpc := rpc.New(config.SolanaConfig.RpcProviders[0])
	rewardAttester := rewards.NewRewardAttester(privateKey, config.Rewards)
	transactionSender := spl.NewTransactionSender(
		config.SolanaConfig.FeePayers,
		config.SolanaConfig.RpcProviders,
	)
	rewardManagerClient, err := reward_manager.NewRewardManagerClient(
		solanaRpc,
		config.SolanaConfig.RewardManagerProgramID,
		config.SolanaConfig.RewardManagerState,
		config.SolanaConfig.RewardManagerLookupTable,
		logger,
	)
	if err != nil {
		panic(err)
	}

	app := &ApiServer{
		App: fiber.New(fiber.Config{
			JSONEncoder:  json.Marshal,
			JSONDecoder:  json.Unmarshal,
			ErrorHandler: errorHandler(logger),
		}),
		pool:                pool,
		queries:             dbv1.New(pool),
		logger:              logger,
		started:             time.Now(),
		resolveHandleCache:  resolveHandleCache,
		resolveGrantCache:   resolveGrantCache,
		rewardAttester:      *rewardAttester,
		transactionSender:   *transactionSender,
		rewardManagerClient: *rewardManagerClient,
		solanaConfig:        config.SolanaConfig,
		antiAbuseOracles:    config.AntiAbuseOracles,
		validators:          config.Nodes,
	}

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	app.Use(cors.New())
	app.Use(RequestTimer())

	app.Use(fiberzap.New(fiberzap.Config{
		Logger: logger,
		FieldsFunc: func(c *fiber.Ctx) []zap.Field {
			fields := []zap.Field{}

			if startTime, ok := c.Locals("start").(time.Time); ok {
				latencyMs := float64(time.Since(startTime).Nanoseconds()) / float64(time.Millisecond)
				fields = append(fields, zap.Float64("latency_ms", latencyMs))
			}

			// Add upstream server to logs, if found
			if upstream, ok := c.Locals("upstream").(string); ok && upstream != "" {
				fields = append(fields, zap.String("upstream", upstream))
			}

			return fields
		},
		Fields: []string{"status", "method", "url", "route"},
	}))

	app.Get("/", app.home)

	// resolve myId
	app.Use(app.isFullMiddleware)
	app.Use(app.resolveMyIdMiddleware)
	app.Use(app.authMiddleware)

	// some not-yet-implemented routes will match handlers below
	// and won't fall thru to python reverse proxy handler
	// so add some exclusions here to make `bridge.audius.co` less broken
	// todo: implement these endpoints in bridgerton.
	{
		app.Use("/v1/full/users/top", BalancerForward(config.PythonUpstreams))
		app.Use("/v1/full/users/genre/top", BalancerForward(config.PythonUpstreams))
		app.Use("/v1/full/users/subscribers", BalancerForward(config.PythonUpstreams))

		app.Use("/v1/full/playlists/top", BalancerForward(config.PythonUpstreams))

		app.Use("/v1/full/tracks/best_new_releases", BalancerForward(config.PythonUpstreams))
		app.Use("/v1/full/tracks/feeling_lucky", BalancerForward(config.PythonUpstreams))
		app.Use("/v1/full/tracks/most_loved", BalancerForward(config.PythonUpstreams))
		app.Use("/v1/full/tracks/remixables", BalancerForward(config.PythonUpstreams))
	}

	v1 := app.Group("/v1")
	v1Full := app.Group("/v1/full")

	for _, g := range []fiber.Router{v1, v1Full} {
		// Users
		g.Get("/users", app.v1Users)
		g.Get("/users/unclaimed_id", app.v1UsersUnclaimedId)
		g.Get("/users/account/:wallet", app.requireAuthMiddleware, app.v1UsersAccount)

		g.Use("/users/handle/:handle", app.requireHandleMiddleware)
		g.Get("/users/handle/:handle", app.v1User)
		g.Get("/users/handle/:handle/tracks", app.v1UserTracks)
		g.Get("/users/handle/:handle/reposts", app.v1UsersReposts)

		g.Use("/users/:userId", app.requireUserIdMiddleware)
		g.Get("/users/:userId", app.v1User)
		g.Get("/users/:userId/challenges", app.v1UsersChallenges)
		g.Get("/users/:userId/comments", app.v1UsersComments)
		g.Get("/users/:userId/followers", app.v1UsersFollowers)
		g.Get("/users/:userId/following", app.v1UsersFollowing)
		g.Get("/users/:userId/library/tracks", app.v1UsersLibraryTracks)
		g.Get("/users/:userId/library/:playlistType", app.v1UsersLibraryPlaylists)
		g.Get("/users/:userId/managers", app.v1UsersManagers)
		g.Get("/users/:userId/managed_users", app.v1UsersManagedUsers)
		g.Get("/users/:userId/mutuals", app.v1UsersMutuals)
		g.Get("/users/:userId/reposts", app.v1UsersReposts)
		g.Get("/users/:userId/related", app.v1UsersRelated)
		g.Get("/users/:userId/supporting", app.v1UsersSupporting)
		g.Get("/users/:userId/supporters", app.v1UsersSupporters)
		g.Get("/users/:userId/tags", app.v1UsersTags)
		g.Get("/users/:userId/tracks", app.v1UserTracks)
		g.Get("/users/:userId/feed", app.v1UsersFeed)
		g.Get("/users/:userId/connected_wallets", app.v1UsersConnectedWallets)

		// Tracks
		g.Get("/tracks", app.v1Tracks)
		g.Get("/tracks/unclaimed_id", app.v1TracksUnclaimedId)

		g.Get("/tracks/trending", app.v1TracksTrending)
		g.Get("/tracks/trending/ids", app.v1TracksTrendingIds)
		g.Get("/tracks/trending/underground", app.v1TracksTrendingUnderground)
		g.Get("/tracks/recommended", app.v1TracksTrending)
		g.Get("/tracks/usdc-purchase", app.v1TracksUsdcPurchase)
		g.Get("/tracks/inspect", app.v1TracksInspect)

		g.Use("/tracks/:trackId", app.requireTrackIdMiddleware)
		g.Get("/tracks/:trackId", app.v1Track)
		g.Get("/tracks/:trackId/stream", app.v1TrackStream)
		g.Get("/tracks/:trackId/download", app.v1TrackDownload)
		g.Get("/tracks/:trackId/inspect", app.v1TrackInspect)
		g.Get("/tracks/:trackId/reposts", app.v1TrackReposts)
		g.Get("/tracks/:trackId/favorites", app.v1TrackFavorites)
		g.Get("/tracks/:trackId/comments", app.v1TrackComments)

		// Playlists
		g.Get("/playlists", app.v1playlists)
		g.Get("/playlists/unclaimed_id", app.v1PlaylistsUnclaimedId)
		g.Get("/playlists/trending", app.v1PlaylistsTrending)

		g.Use("/playlists/:playlistId", app.requirePlaylistIdMiddleware)
		g.Get("/playlists/:playlistId", app.v1Playlist)
		g.Get("/playlists/:playlistId/reposts", app.v1PlaylistReposts)
		g.Get("/playlists/:playlistId/favorites", app.v1PlaylistFavorites)

		// Developer Apps
		g.Get("/developer_apps/:address", app.v1DeveloperApps)

		// Rewards
		g.Post("/rewards/claim", app.v1ClaimRewards)

		// Resolve
		g.Get("/resolve", app.v1Resolve)

		// Comments
		g.Get("/comments/unclaimed_id", app.v1CommentsUnclaimedId)

		// Events
		g.Get("/events/unclaimed_id", app.v1EventsUnclaimedId)
		g.Get("/events", app.v1Events)
		g.Get("/events/all", app.v1Events)
		g.Get("/events/entity", app.v1Events)

		// Challenges
		g.Get("/challenges/undisbursed", app.v1ChallengesUndisbursed)

		// Metrics
		g.Get("/metrics/genres", app.v1GenreMetrics)
		g.Get("/metrics/plays", app.v1PlaysMetrics)
		g.Get("/metrics/aggregates/apps/:time_range", app.v1AppAggregateMetrics)
	}

	app.Static("/", "./static")

	// proxy unhandled requests thru to existing discovery API
	app.Use(BalancerForward(config.PythonUpstreams))

	// gracefully handle 404
	// (this won't get hit so long as above proxy is in place)
	app.Use(func(c *fiber.Ctx) error {
		return sendError(c, 404, "Route not found")
	})

	return app
}

type ApiServer struct {
	*fiber.App
	pool                *pgxpool.Pool
	queries             *dbv1.Queries
	logger              *zap.Logger
	started             time.Time
	resolveHandleCache  otter.Cache[string, int32]
	resolveGrantCache   otter.Cache[string, bool]
	rewardManagerClient reward_manager.RewardManagerClient
	rewardAttester      rewards.RewardAttester
	transactionSender   spl.TransactionSender
	solanaConfig        config.SolanaConfig
	antiAbuseOracles    []string
	validators          []config.Node
}

func (app *ApiServer) home(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"env":     config.Cfg.Env,
		"started": app.started,
		"uptime":  time.Since(app.started).Truncate(time.Second).String(),
	})
}

func decodeIdList(c *fiber.Ctx) []int32 {
	var ids []int32
	for _, b := range c.Request().URI().QueryArgs().PeekMulti("id") {
		if id, err := trashid.DecodeHashId(string(b)); err == nil {
			ids = append(ids, int32(id))
		}
		if id, err := strconv.Atoi(string(b)); err == nil {
			ids = append(ids, int32(id))
		}
	}
	return ids
}

func queryMutli(c *fiber.Ctx, key string) []string {
	var values []string
	for _, v := range c.Request().URI().QueryArgs().PeekMulti(key) {
		values = append(values, string(v))
	}
	return values
}

var validDateBuckets = map[string]bool{
	"hour":   true,
	"day":    true,
	"week":   true,
	"month":  true,
	"year":   true,
	"minute": true,
}

func (app *ApiServer) queryDateBucket(c *fiber.Ctx, param string, defaultValue string) (string, error) {
	bucket := c.Query(param, defaultValue)
	if !validDateBuckets[bucket] {
		return "", fmt.Errorf("invalid %s parameter: %s", param, bucket)
	}
	return bucket, nil
}

var validTimeRanges = map[string]bool{
	"week":     true,
	"month":    true,
	"year":     true,
	"all_time": true,
}

func (app *ApiServer) paramTimeRange(c *fiber.Ctx, param string, defaultValue string) (string, error) {
	timeRange := c.Params(param, defaultValue)
	if !validTimeRanges[timeRange] {
		return "", fmt.Errorf("invalid %s parameter: %s", param, timeRange)
	}
	return timeRange, nil
}

func (app *ApiServer) resolveUserHandleToId(handle string) (int32, error) {
	if hit, ok := app.resolveHandleCache.Get(handle); ok {
		return hit, nil
	}
	user_id, err := app.queries.GetUserForHandle(context.Background(), handle)
	if err != nil {
		return 0, err
	}
	app.resolveHandleCache.Set(handle, int32(user_id))
	return int32(user_id), nil
}

func (as *ApiServer) Serve() {
	flushTicker := time.NewTicker(time.Second * 15)
	go func() {
		for range flushTicker.C {
			as.logger.Sync()
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		flushTicker.Stop()
		as.Shutdown()
		as.pool.Close()
		as.logger.Sync()
	}()

	if err := as.Listen(":1323"); err != nil && err != http.ErrServerClosed {
		as.logger.Fatal("Failed to start server", zap.Error(err))
	}
}
