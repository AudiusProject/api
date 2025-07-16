package api

import (
	"context"
	_ "embed"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/esindexer"
	"bridgerton.audius.co/logging"
	"bridgerton.audius.co/solana/spl"
	"bridgerton.audius.co/solana/spl/programs/claimable_tokens"
	"bridgerton.audius.co/solana/spl/programs/reward_manager"
	"bridgerton.audius.co/trashid"
	"github.com/AudiusProject/audiusd/pkg/rewards"
	"github.com/AudiusProject/audiusd/pkg/sdk"
	"github.com/Doist/unfurlist"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gofiber/contrib/fiberzap/v2"
	"github.com/gofiber/contrib/swagger"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/fiber/v2/utils"
	pgxzap "github.com/jackc/pgx-zap"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/maypok86/otter"
	"github.com/mcuadros/go-defaults"
	"github.com/segmentio/encoding/json"
	"go.uber.org/zap"
)

//go:embed swagger/swagger-v1.yaml
var swaggerV1 []byte

//go:embed swagger/swagger-v1-full.yaml
var swaggerV1Full []byte

func RequestTimer() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.Locals("start", time.Now())
		return c.Next()
	}
}

func NewApiServer(config config.Config) *ApiServer {
	logger := logging.NewZapLogger(config).
		With(zap.String("service", "ApiServer"))
	requestValidator := initRequestValidator()

	connConfig, err := pgxpool.ParseConfig(config.ReadDbUrl)
	if err != nil {
		logger.Error("read db connect failed", zap.Error(err))
	}

	// register enum types with connection
	// this is mostly to support COPY protocol as used by tests
	// connConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
	// 	enumNames := []string{"challengetype"}
	// 	for _, name := range enumNames {
	// 		typ, err := conn.LoadType(ctx, name)
	// 		if err != nil {
	// 			panic(err)
	// 		}
	// 		conn.TypeMap().RegisterType(typ)
	// 	}
	// 	return nil
	// }

	// disable sql logging in ENV "test"
	if config.Env != "test" {
		connConfig.ConnConfig.Tracer = &tracelog.TraceLog{
			Logger:   pgxzap.NewLogger(logger),
			LogLevel: logging.GetTraceLogLevel(config.LogLevel),
		}
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), connConfig)

	if err != nil {
		logger.Fatal("read db connect failed", zap.Error(err))
	}

	// Set up write database connection
	var writePool *pgxpool.Pool
	if config.WriteDbUrl != "" {
		writeConnConfig, err := pgxpool.ParseConfig(config.WriteDbUrl)
		if err != nil {
			logger.Error("write db connect failed", zap.Error(err))
		}

		if config.Env != "test" {
			writeConnConfig.ConnConfig.Tracer = &tracelog.TraceLog{
				Logger:   pgxzap.NewLogger(logger),
				LogLevel: logging.GetTraceLogLevel(config.LogLevel),
			}
		}

		writePool, err = pgxpool.NewWithConfig(context.Background(), writeConnConfig)
		if err != nil {
			logger.Fatal("write db connect failed", zap.Error(err))
		}
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

	resolveWalletCache, err := otter.MustBuilder[string, int](50_000).
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
	claimableTokensClient, err := claimable_tokens.NewClaimableTokensClient(
		solanaRpc,
		config.SolanaConfig.ClaimableTokensProgramID,
		transactionSender,
	)
	if err != nil {
		panic(err)
	}

	esClient, err := esindexer.Dial(config.EsUrl)
	if err != nil {
		logger.Error("dial es failed", zap.String("url", config.EsUrl), zap.Error(err))
	}

	auds := sdk.NewAudiusdSDK(config.AudiusdURL)

	skipAuthCheck, _ := strconv.ParseBool(os.Getenv("skipAuthCheck"))

	// Initialize metrics collector if writePool is available
	var metricsCollector *MetricsCollector
	if writePool != nil {
		metricsCollector = NewMetricsCollector(logger, writePool)
	}

	app := &ApiServer{
		App: fiber.New(fiber.Config{
			JSONEncoder:    json.Marshal,
			JSONDecoder:    json.Unmarshal,
			ErrorHandler:   errorHandler(logger),
			ReadBufferSize: 32_768,
			UnescapePath:   true,
		}),
		env:                   config.Env,
		skipAuthCheck:         skipAuthCheck,
		pool:                  pool,
		writePool:             writePool,
		queries:               dbv1.New(pool),
		logger:                logger,
		esClient:              esClient,
		started:               time.Now(),
		resolveHandleCache:    &resolveHandleCache,
		resolveGrantCache:     &resolveGrantCache,
		resolveWalletCache:    &resolveWalletCache,
		requestValidator:      requestValidator,
		rewardAttester:        rewardAttester,
		transactionSender:     transactionSender,
		rewardManagerClient:   rewardManagerClient,
		claimableTokensClient: claimableTokensClient,
		solanaConfig:          &config.SolanaConfig,
		antiAbuseOracles:      config.AntiAbuseOracles,
		validators:            config.Nodes,
		auds:                  auds,
		metricsCollector:      metricsCollector,
	}

	// Set up a custom decoder for HashIds so they can be parsed in lists
	// used in query parameters. Without this, the decoder doesn't know what
	// to do to parse them into trashid.HashIds
	fiber.SetParserDecoder(fiber.ParserConfig{
		SetAliasTag:       "query",
		IgnoreUnknownKeys: true, // same as default
		ZeroEmpty:         true, // same as default
		ParserType: []fiber.ParserType{{
			Customtype: trashid.HashId(0),
			Converter: func(s string) reflect.Value {
				id, err := trashid.DecodeHashId(s)
				if err != nil {
					// Return 0 when failing to decode
					return reflect.Zero(reflect.TypeOf(trashid.HashId(0)))
				}
				return reflect.ValueOf(trashid.HashId(id))
			},
		}},
	})

	app.Use(recover.New(recover.Config{
		EnableStackTrace: true,
	}))

	app.Use(cors.New())
	app.Use(RequestTimer())
	app.Use(requestid.New(requestid.Config{
		Next:       nil,
		Header:     fiber.HeaderXRequestID,
		Generator:  utils.UUIDv4,
		ContextKey: "requestId",
	}))

	// Add request metrics middleware if available
	if app.metricsCollector != nil {
		app.Use(app.metricsCollector.Middleware())
	}
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

			if requestId, ok := c.Locals("requestId").(string); ok && requestId != "" {
				fields = append(fields, zap.String("request_id", requestId))
			}

			return fields
		},
		Fields: []string{"status", "method", "url", "route"},
	}))

	app.Get("/", app.home)

	// Debug endpoints
	app.Get("/debug/es", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"es_url": config.EsUrl,
		})
	})
	app.Get("/debug/metrics", func(c *fiber.Ctx) error {
		if app.metricsCollector == nil {
			return c.JSON(fiber.Map{
				"error": "metrics collector not initialized",
			})
		}
		return c.JSON(app.metricsCollector.Debug())
	})

	// resolve myId
	app.Use(app.isFullMiddleware)
	app.Use(app.resolveMyIdMiddleware)
	app.Use(app.authMiddleware)

	// some not-yet-implemented routes will match handlers below
	// and won't fall thru to python reverse proxy handler
	// so add some exclusions here to make `bridge.audius.co` less broken
	// todo: implement these endpoints in bridgerton.
	{
		app.Use("/v1/full/playlists/top", BalancerForward(config.PythonUpstreams))

		app.Use("/v1/full/tracks/best_new_releases", BalancerForward(config.PythonUpstreams))
		app.Use("/v1/full/tracks/most_loved", BalancerForward(config.PythonUpstreams))
		app.Use("/v1/full/tracks/remixables", BalancerForward(config.PythonUpstreams))
	}

	v1 := app.Group("/v1")
	v1Full := app.Group("/v1/full")

	for _, g := range []fiber.Router{v1, v1Full} {
		// Users
		g.Get("/users", app.v1Users)
		g.Get("/users/search", app.v1UsersSearch)
		g.Get("/users/unclaimed_id", app.v1UsersUnclaimedId)
		g.Get("/users/top", app.v1UsersTop)
		g.Get("/users/genre/top", app.v1UsersGenreTop)
		g.Get("/users/account/:wallet", app.requireAuthMiddleware, app.v1UsersAccount)

		g.Use("/users/handle/:handle", app.requireHandleMiddleware)
		g.Get("/users/handle/:handle", app.v1User)
		g.Get("/users/handle/:handle/tracks", app.v1UserTracks)
		g.Get("/users/handle/:handle/albums", app.v1UserAlbums)
		g.Get("/users/handle/:handle/playlists", app.v1UserPlaylists)
		g.Get("/users/handle/:handle/tracks/ai_attributed", app.v1UserTracksAiAttributed)
		g.Get("/users/handle/:handle/reposts", app.v1UsersReposts)

		g.Use("/users/:userId", app.requireUserIdMiddleware)
		g.Get("/users/:userId", app.v1User)
		g.Get("/users/:userId/challenges", app.v1UsersChallenges)
		g.Get("/users/:userId/comments", app.v1UsersComments)
		g.Get("/users/:userId/followers", app.v1UsersFollowers)
		g.Get("/users/:userId/following", app.v1UsersFollowing)
		g.Get("/users/:userId/favorites", app.v1UsersFavorites)
		g.Get("/users/:userId/library/tracks", app.v1UsersLibraryTracks)
		g.Get("/users/:userId/library/:playlistType", app.v1UsersLibraryPlaylists)
		g.Get("/users/:userId/managers", app.v1UsersManagers)
		g.Get("/users/:userId/managed_users", app.v1UsersManagedUsers)
		g.Get("/users/:userId/mutuals", app.v1UsersMutuals)
		g.Get("/users/:userId/reposts", app.v1UsersReposts)
		g.Get("/users/:userId/related", app.v1UsersRelated)
		g.Get("/users/:userId/supporting", app.v1UsersSupporting)
		g.Get("/users/:userId/supporting/:supportedUserId", app.v1UsersSupporting)
		g.Get("/users/:userId/supporters", app.v1UsersSupporters)
		g.Get("/users/:userId/supporters/:supporterUserId", app.v1UsersSupporters)
		g.Get("/users/:userId/tags", app.v1UsersTags)
		g.Get("/users/:userId/tracks", app.v1UserTracks)
		g.Get("/users/:userId/albums", app.v1UserAlbums)
		g.Get("/users/:userId/playlists", app.v1UserPlaylists)
		g.Get("/users/:userId/feed", app.v1UsersFeed)
		g.Get("/users/:userId/connected_wallets", app.v1UsersConnectedWallets)
		g.Get("/users/:userId/transactions/audio", app.v1UsersTransactionsAudio)
		g.Get("/users/:userId/transactions/audio/count", app.v1UsersTransactionsAudioCount)
		g.Get("/users/:userId/transactions/usdc", app.v1UsersTransactionsUsdc)
		g.Get("/users/:userId/transactions/usdc/count", app.v1UsersTransactionsUsdcCount)
		g.Get("/users/:userId/history/tracks", app.v1UsersHistory)
		g.Get("/users/:userId/listen_counts_monthly", app.v1UsersListenCountsMonthly)
		g.Get("/users/:userId/purchases", app.v1UsersPurchases)
		g.Get("/users/:userId/purchases/count", app.v1UsersPurchasesCount)
		g.Get("/users/:userId/sales", app.v1UsersSales)
		g.Get("/users/:userId/sales/count", app.v1UsersSalesCount)
		g.Get("/users/:userId/muted", app.v1UsersMuted)
		g.Get("/users/:userId/subscribers", app.v1UsersSubscribers)
		g.Get("/users/:userId/recommended-tracks", app.v1UsersRecommendedTracks)
		g.Get("/users/:userId/now-playing", app.v1UsersNowPlaying)

		// Tracks
		g.Get("/tracks", app.v1Tracks)
		g.Get("/tracks/search", app.v1TracksSearch)
		g.Get("/tracks/unclaimed_id", app.v1TracksUnclaimedId)

		g.Get("/tracks/trending", app.v1TracksTrending)
		g.Get("/tracks/trending/ids", app.v1TracksTrendingIds)
		g.Get("/tracks/trending/underground", app.v1TracksTrendingUnderground)
		g.Get("/tracks/recommended", app.v1TracksTrending)
		g.Get("/tracks/recent-premium", app.v1TracksRecentPremium)
		g.Get("/tracks/usdc-purchase", app.v1TracksUsdcPurchase)
		g.Get("/tracks/inspect", app.v1TracksInspect)
		g.Get("/tracks/feeling-lucky", app.v1TracksFeelingLucky)
		g.Get("/tracks/recent-comments", app.v1TracksRecentComments)
		g.Get("/tracks/most-shared", app.v1TracksMostShared)

		g.Use("/tracks/:trackId", app.requireTrackIdMiddleware)
		g.Get("/tracks/:trackId", app.v1Track)
		g.Get("/tracks/:trackId/stream", app.v1TrackStream)
		g.Get("/tracks/:trackId/download", app.v1TrackDownload)
		g.Get("/tracks/:trackId/inspect", app.v1TrackInspect)
		g.Get("/tracks/:trackId/remixes", app.v1TrackRemixes)
		g.Get("/tracks/:trackId/reposts", app.v1TrackReposts)
		g.Get("/tracks/:trackId/stems", app.v1TrackStems)
		g.Get("/tracks/:trackId/favorites", app.v1TrackFavorites)
		g.Get("/tracks/:trackId/comments", app.v1TrackComments)
		g.Get("/tracks/:trackId/comment_count", app.v1TrackCommentCount)
		g.Get("/tracks/:trackId/comment_notification_setting", app.v1TrackCommentNotificationSetting)
		g.Get("/tracks/:trackId/remixing", app.v1TrackRemixing)
		g.Get("/tracks/:trackId/top_listeners", app.v1TrackTopListeners)

		// Playlists
		g.Get("/playlists", app.v1Playlists)
		g.Get("/playlists/search", app.v1PlaylistsSearch)
		g.Get("/playlists/unclaimed_id", app.v1PlaylistsUnclaimedId)
		g.Get("/playlists/trending", app.v1PlaylistsTrending)
		g.Get("/playlists/by_permalink/:handle/:slug", app.v1PlaylistByPermalink)

		g.Use("/playlists/:playlistId", app.requirePlaylistIdMiddleware)
		g.Get("/playlists/:playlistId", app.v1Playlist)
		g.Get("/playlists/:playlistId/stream", app.v1PlaylistStream)
		g.Get("/playlists/:playlistId/reposts", app.v1PlaylistReposts)
		g.Get("/playlists/:playlistId/favorites", app.v1PlaylistFavorites)

		// Explore
		g.Get("/explore/best-selling", app.v1ExploreBestSelling)

		// Search
		g.Get("/search/autocomplete", app.v1SearchFull)
		g.Get("/search/full", app.v1SearchFull)
		g.Get("/search/tags", app.v1SearchFull)

		// Developer Apps
		g.Get("/developer_apps/:address", app.v1DeveloperApps)

		// Rewards
		g.Post("/rewards/claim", app.v1ClaimRewards)

		// Resolve
		g.Get("/resolve", app.v1Resolve)

		// Comments
		g.Get("/comments/unclaimed_id", app.v1CommentsUnclaimedId)
		g.Get("/comments/:commentId", app.v1Comment)

		// Events
		g.Get("/events/unclaimed_id", app.v1EventsUnclaimedId)
		g.Get("/events", app.v1Events)
		g.Get("/events/all", app.v1Events)
		g.Get("/events/entity", app.v1Events)

		// Challenges
		g.Get("/challenges/undisbursed", app.v1ChallengesUndisbursed)

		// Metrics
		g.Get("/metrics/genres", app.v1MetricsGenres)
		g.Get("/metrics/plays", app.v1MetricsPlays)
		g.Get("/metrics/aggregates/apps/:time_range", app.v1MetricsApps)
		g.Get("/metrics/aggregates/routes/:time_range", app.v1MetricsRoutes)
		g.Get("/metrics/aggregates/routes/trailing/:time_range", app.v1MetricsRoutesTrailing)

		// Notifications
		g.Get("/notifications/:userId/playlist_updates", app.requireUserIdMiddleware, app.v1NotificationsPlaylistUpdates)

	}

	// Comms
	comms := app.Group("/comms")
	// Cached/non-cached are the same as there are no other nodes to query anymore
	comms.Get("/pubkey/:userId", app.requireUserIdMiddleware, app.getPubkey)
	comms.Get("/pubky/:userId/cached", app.requireUserIdMiddleware, app.getPubkey)

	unfurlBlocklist := unfurlist.WithBlocklistPrefixes(
		[]string{
			"http://localhost",
			"http://127",
			"http://10",
			"http://169.254",
			"http://172.16",
			"http://192.168",
			"http://::1",
			"http://fe80::",
		},
	)
	unfurlHeaders := unfurlist.WithExtraHeaders(map[string]string{"User-Agent": "twitterbot"})
	comms.Get("/unfurl", adaptor.HTTPHandler(unfurlist.New(unfurlBlocklist, unfurlHeaders)))

	comms.Get("/chats", app.getChats)
	comms.Get("/chats/unread", app.getUnreadCount)
	comms.Get("/chats/permissions", app.getChatPermissions)
	comms.Get("/chats/blockers", app.getChatBlockers)
	comms.Get("/chats/blockees", app.getChatBlockees)

	comms.Get("/chats/:chatId/messages", app.getChatMessages)
	comms.Get("/chats/:chatId", app.getChat)

	comms.Get("/blasts", app.getNewBlasts)

	// Block confirmation
	app.Get("/block_confirmation", app.BlockConfirmation)

	app.Static("/", "./static")

	// Disable swagger in test environments, because it will slow things down a lot
	if config.Env != "test" {
		// Create Swagger middleware for v1
		//
		// Swagger will be available at: /v1
		app.Use(swagger.New(swagger.Config{
			BasePath: "/",
			Path:     "v1",
			// Only controls where the swagger.json is server from
			FilePath:    "v1/swagger.yaml",
			FileContent: swaggerV1,
		}))

		// Create Swagger middleware for v1/full
		//
		// Swagger will be available at: /v1/full
		app.Use(swagger.New(swagger.Config{
			BasePath: "/",
			Path:     "v1/full",
			// Only controls where the swagger.json is server from
			FilePath:    "v1/full/swagger.yaml",
			FileContent: swaggerV1Full,
		}))
	}

	// proxy unhandled requests thru to existing discovery API
	app.Use(BalancerForward(config.PythonUpstreams))

	// gracefully handle 404
	// (this won't get hit so long as above proxy is in place)
	app.Use(func(c *fiber.Ctx) error {
		return fiber.ErrNotFound
	})

	return app
}

type ApiServer struct {
	*fiber.App
	pool                  *pgxpool.Pool
	writePool             *pgxpool.Pool
	queries               *dbv1.Queries
	esClient              *elasticsearch.Client
	logger                *zap.Logger
	started               time.Time
	resolveHandleCache    *otter.Cache[string, int32]
	resolveGrantCache     *otter.Cache[string, bool]
	resolveWalletCache    *otter.Cache[string, int]
	requestValidator      *RequestValidator
	rewardManagerClient   *reward_manager.RewardManagerClient
	claimableTokensClient *claimable_tokens.ClaimableTokensClient
	rewardAttester        *rewards.RewardAttester
	transactionSender     *spl.TransactionSender
	solanaConfig          *config.SolanaConfig
	antiAbuseOracles      []string
	validators            []config.Node
	env                   string
	auds                  *sdk.AudiusdSDK
	skipAuthCheck         bool // set to true in a test if you don't care about auth middleware
	metricsCollector      *MetricsCollector
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

		// Shutdown metrics collector if it exists
		if as.metricsCollector != nil {
			as.metricsCollector.Shutdown()
		}

		// Shutdown HLL aggregator if it exists
		// Removed hllAggregator

		as.Shutdown()
		as.pool.Close()
		if as.writePool != nil {
			as.writePool.Close()
		}
		as.logger.Sync()
	}()

	if err := as.Listen(":1323"); err != nil && err != http.ErrServerClosed {
		as.logger.Fatal("Failed to start server", zap.Error(err))
	}
}

// Move this to a new module if we add custom validation
func (as *ApiServer) ParseAndValidateQueryParams(c *fiber.Ctx, v any) error {
	if err := c.QueryParser(v); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	defaults.SetDefaults(v)
	return as.requestValidator.Validate(v)
}
