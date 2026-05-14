package api

import (
	"context"
	"crypto/tls"
	_ "embed"
	"net"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"strconv"
	"time"

	comms "api.audius.co/api/comms"
	"api.audius.co/api/dbv1"
	"api.audius.co/birdeye"
	"api.audius.co/config"
	"api.audius.co/esindexer"
	"api.audius.co/logging"
	"api.audius.co/solana/spl"
	"api.audius.co/solana/spl/programs/claimable_tokens"
	"api.audius.co/solana/spl/programs/meteora_dbc"
	"api.audius.co/solana/spl/programs/reward_manager"
	"api.audius.co/trashid"
	apiutils "api.audius.co/utils"
	"github.com/Doist/unfurlist"
	"github.com/OpenAudio/go-openaudio/pkg/rewards"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gofiber/contrib/fiberzap/v2"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/fiber/v2/middleware/requestid"
	"github.com/gofiber/fiber/v2/utils"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"github.com/maypok86/otter"
	"github.com/mcuadros/go-defaults"
	"github.com/segmentio/encoding/json"
	"go.uber.org/zap"
)

//go:embed swagger/swagger-v1.yaml
var swaggerV1 []byte

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

	// Create DBPools from read replicas
	var connectionStrings []string
	if len(config.ReadDbReplicas) > 0 && config.ReadDbReplicas[0] != "" {
		// Use read replicas if configured
		connectionStrings = config.ReadDbReplicas
	} else {
		// Fall back to single read database
		connectionStrings = []string{config.ReadDbUrl}
	}

	pool, err := dbv1.NewDBPools(connectionStrings, logger, config.Env, config.ZapLevel)
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
				Logger:   logging.NewSqlLogger(logger, config.ZapLevel),
				LogLevel: tracelog.LogLevelTrace, // capture everything into sql logger
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

	apiAccessKeySignerCache, err := otter.MustBuilder[string, apiAccessKeySignerEntry](10_000).
		WithTTL(5 * time.Minute).
		CollectStats().
		Build()
	if err != nil {
		panic(err)
	}

	oauthTokenCache, err := otter.MustBuilder[string, oauthTokenCacheEntry](10_000).
		WithTTL(60 * time.Second).
		CollectStats().
		Build()
	if err != nil {
		panic(err)
	}

	// Holds at most ~4 entries (one per Type/Time combination used by
	// /v1/playlists/trending). The "qualified" set is request-independent
	// and the underlying query takes 3+ seconds, so a short TTL trades
	// trivial staleness for huge p95 wins.
	qualifiedPlaylistsCache, err := otter.MustBuilder[string, []int32](32).
		WithTTL(5 * time.Minute).
		CollectStats().
		Build()
	if err != nil {
		panic(err)
	}

	// Caches the candidate user-id list returned by the /v1/users/:userId/
	// related collaborative-filter / genre-fallback query. The list is a
	// recommendation, not authoritative state, so a 10-minute TTL is fine.
	relatedUsersCache, err := otter.MustBuilder[string, []int32](20_000).
		WithTTL(10 * time.Minute).
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
	meteoraDbcClient := meteora_dbc.NewClient(
		solanaRpc,
		logger,
	)

	esClient, err := esindexer.Dial(config.EsUrl)
	if err != nil {
		logger.Error("dial es failed", zap.String("url", config.EsUrl), zap.Error(err))
	}

	httpClient := http.DefaultClient
	if config.Env == "development" || config.Env == "dev" {
		httpClient = &http.Client{
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   30 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				ForceAttemptHTTP2:     true,
				MaxIdleConns:          100,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ExpectContinueTimeout: 1 * time.Second,
				// Skip TLS verification in dev mode
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}

	openAudioSDK := sdk.NewOpenAudioSDKWithClient(config.AudiusdURL, httpClient)
	// Build OpenAudio pool URLs; fall back to single URL if none provided
	openAudioURLs := config.OpenAudioURLs
	if len(openAudioURLs) == 0 {
		openAudioURLs = []string{config.AudiusdURL}
	}
	openAudioPool := NewOpenAudioPool(openAudioURLs, httpClient)

	skipAuthCheck, _ := strconv.ParseBool(os.Getenv("skipAuthCheck"))

	// Initialize metrics collector if writePool is available
	var metricsCollector *MetricsCollector
	var rateLimitMiddleware *RateLimitMiddleware
	if writePool != nil && config.Env != "test" {
		metricsCollector = NewMetricsCollector(logger, writePool)
		isLocalDev := config.Env == "dev" || config.Env == "development" || config.Env == ""
		if !isLocalDev {
			rateLimitMiddleware = NewRateLimitMiddleware(logger, writePool)
		}
	}

	commsRpcProcessor, err := comms.NewProcessor(pool, writePool, &config, logger)
	if err != nil {
		panic(err)
	}

	app := &ApiServer{
		App: fiber.New(fiber.Config{
			JSONEncoder:    json.Marshal,
			JSONDecoder:    json.Unmarshal,
			ErrorHandler:   errorHandler(logger),
			ReadBufferSize: 32_768,
			UnescapePath:   true,
		}),
		config:                  &config,
		commsRpcProcessor:       commsRpcProcessor,
		env:                     config.Env,
		audiusAppUrl:            config.AudiusAppUrl,
		skipAuthCheck:           skipAuthCheck,
		pool:                    pool,
		writePool:               writePool,
		queries:                 dbv1.New(pool),
		logger:                  logger,
		esClient:                esClient,
		started:                 time.Now(),
		resolveHandleCache:      &resolveHandleCache,
		resolveGrantCache:       &resolveGrantCache,
		resolveWalletCache:      &resolveWalletCache,
		apiAccessKeySignerCache: &apiAccessKeySignerCache,
		oauthTokenCache:         &oauthTokenCache,
		qualifiedPlaylistsCache: &qualifiedPlaylistsCache,
		relatedUsersCache:       &relatedUsersCache,
		requestValidator:        requestValidator,
		rewardAttester:          rewardAttester,
		transactionSender:       transactionSender,
		rewardManagerClient:     rewardManagerClient,
		claimableTokensClient:   claimableTokensClient,
		solanaConfig:            &config.SolanaConfig,
		antiAbuseOracles:        config.AntiAbuseOracles,
		validators:              NewNodes(),
		openAudioSDK:            openAudioSDK,
		openAudioPool:           openAudioPool,
		metricsCollector:        metricsCollector,
		rateLimitMiddleware:     rateLimitMiddleware,
		birdeyeClient:           birdeye.New(config.BirdeyeToken),
		solanaRpcClient:         solanaRpc,
		meteoraDbcClient:        meteoraDbcClient,
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
		app.Use(app.metricsCollector.Middleware(app))
	}
	// Add rate limit middleware after metrics, before auth
	if app.rateLimitMiddleware != nil {
		app.Use(app.rateLimitMiddleware.Middleware(app))
	}
	// Avoid request log spam in tests; test failures still include assertion output.
	if config.Env != "test" {
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

				ipAddress := apiutils.GetIP(c)
				fields = append(fields, zap.String("ip", ipAddress))

				return fields
			},
			Fields: []string{"status", "method", "url", "route"},
		}))
	}

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

	// Unsplash proxy
	app.All("/unsplash/*", app.unsplash)

	// Archiver proxy
	app.All("/archive/*", archiveProxy)

	// Sitemaps
	app.Get("/sitemaps/default.xml", app.sitemapDefault)
	app.Get("/sitemaps/defaults.xml", app.sitemapDefaults)
	app.Get("/sitemaps/:type/index.xml", app.sitemapTypeIndex)
	app.Get("/sitemaps/:type/:fileName", app.sitemapTypePage)

	// resolve myId
	app.Use(app.isFullMiddleware)
	app.Use(app.resolveMyIdMiddleware)
	app.Use(app.authMiddleware)
	app.Use(app.solanaWalletMiddleware)
	app.Use(app.id3Middleware)

	v1 := app.Group("/v1")
	v1Full := app.Group("/v1/full")

	for _, g := range []fiber.Router{v1, v1Full} {
		// Users
		g.Get("/users", app.v1Users)
		g.Post("/users", app.requireAuthMiddleware, app.requireWriteScope, app.postV1Users)
		g.Get("/users/address", app.v1UserIdsByAddresses)
		g.Get("/users/search", app.v1UsersSearch)
		g.Get("/users/unclaimed_id", app.v1UsersUnclaimedId)
		g.Get("/users/top", app.v1UsersTop)
		g.Get("/users/genre/top", app.v1UsersGenreTop)
		g.Get("/users/account/:wallet", app.requireAuthMiddleware, app.v1UsersAccount)
		g.Get("/users/verify_token", app.v1UsersVerifyToken)

		g.Use("/users/handle/:handle", app.requireHandleMiddleware)
		g.Get("/users/handle/:handle", app.v1User)
		g.Get("/users/handle/:handle/tracks", app.v1UserTracks)
		g.Get("/users/handle/:handle/tracks/count", app.v1UserTracksCount)
		g.Get("/users/handle/:handle/albums", app.v1UserAlbums)
		g.Get("/users/handle/:handle/contests", app.v1UserContests)
		g.Get("/users/handle/:handle/playlists", app.v1UserPlaylists)
		g.Get("/users/handle/:handle/tracks/ai_attributed", app.v1UserTracksAiAttributed)
		g.Get("/users/handle/:handle/tracks/ai-attributed", app.v1UserTracksAiAttributed)
		g.Get("/users/handle/:handle/reposts", app.v1UsersReposts)

		g.Use("/users/:userId", app.requireUserIdMiddleware)
		g.Get("/users/:userId", app.v1User)
		g.Get("/users/:userId/challenges", app.v1UsersChallenges)
		g.Get("/users/:userId/collectibles", app.v1UsersCollectibles)
		g.Get("/users/:userId/comments", app.v1UsersComments)
		g.Get("/users/:userId/emails/:grantorUserId/key", app.v1UsersEmailsKey)
		g.Get("/users/:userId/followers", app.v1UsersFollowers)
		g.Get("/users/:userId/following", app.v1UsersFollowing)
		g.Get("/users/:userId/favorites", app.v1UsersFavorites)
		g.Get("/users/:userId/library/tracks", app.v1UsersLibraryTracks)
		g.Get("/users/:userId/library/:playlistType", app.v1UsersLibraryPlaylists)
		g.Get("/users/:userId/balance/history", app.v1UsersBalanceHistory)
		g.Get("/users/:userId/managers", app.v1UsersManagers)
		g.Get("/users/:userId/managed_users", app.v1UsersManagedUsers)
		g.Get("/grantees/:address/users", app.v1GranteeUsers)
		g.Post("/users/:userId/grants", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UsersGrant)
		g.Delete("/users/:userId/grants/:address", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1UsersGrant)
		g.Post("/users/:userId/managers", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UsersManager)
		g.Delete("/users/:userId/managers/:managerUserId", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1UsersManager)
		g.Post("/users/:userId/grants/approve", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UsersApproveGrant)
		g.Get("/users/:userId/mutuals", app.v1UsersMutuals)
		g.Get("/users/:userId/reposts", app.v1UsersReposts)
		g.Get("/users/:userId/related", app.v1UsersRelated)
		g.Get("/users/:userId/supporting", app.v1UsersSupporting)
		g.Get("/users/:userId/supporting/:supportedUserId", app.v1UsersSupporting)
		g.Get("/users/:userId/supporters", app.v1UsersSupporters)
		g.Get("/users/:userId/supporters/:supporterUserId", app.v1UsersSupporters)
		g.Get("/users/:userId/tags", app.v1UsersTags)
		g.Get("/users/:userId/tracks", app.v1UserTracks)
		g.Get("/users/:userId/tracks/count", app.v1UserTracksCount)
		g.Get("/users/:userId/tracks/download_count", app.v1UserTracksDownloadCount)
		g.Get("/users/:userId/tracks/remixed", app.v1UserTracksRemixed)
		g.Get("/users/:userId/albums", app.v1UserAlbums)
		g.Get("/users/:userId/contests", app.v1UserContests)
		g.Get("/users/:userId/playlists", app.v1UserPlaylists)
		g.Get("/users/:userId/feed", app.v1UsersFeed)
		g.Get("/users/:userId/connected_wallets", app.v1UsersConnectedWallets)
		g.Get("/users/:userId/transactions/audio", app.v1UsersTransactionsAudio)
		g.Get("/users/:userId/transactions/audio/count", app.v1UsersTransactionsAudioCount)
		g.Get("/users/:userId/transactions/usdc", app.v1UsersTransactionsUsdc)
		g.Get("/users/:userId/transactions/usdc/count", app.v1UsersTransactionsUsdcCount)
		g.Get("/users/:userId/history/tracks", app.v1UsersHistory)
		g.Get("/users/:userId/listen_counts_monthly", app.v1UsersListenCountsMonthly)
		g.Get("/users/:userId/purchases", app.requireAuthForUserId, app.v1UsersPurchases)
		g.Get("/users/:userId/purchases/download", app.requireAuthForUserId, app.v1UsersPurchasesDownloadCsv)
		g.Get("/users/:userId/purchases/download/json", app.requireAuthForUserId, app.v1UsersPurchasesDownloadJson)
		g.Get("/users/:userId/purchases/count", app.requireAuthForUserId, app.v1UsersPurchasesCount)
		g.Get("/users/:userId/sales", app.requireAuthForUserId, app.v1UsersSales)
		g.Get("/users/:userId/sales/download", app.requireAuthForUserId, app.v1UsersSalesDownloadCsv)
		g.Get("/users/:userId/sales/download/json", app.requireAuthForUserId, app.v1UsersSalesDownloadJson)
		g.Get("/users/:userId/sales/count", app.requireAuthForUserId, app.v1UsersSalesCount)
		g.Get("/users/:userId/sales/aggregate", app.requireAuthForUserId, app.v1UsersSalesAggregate)
		g.Get("/users/:userId/muted", app.v1UsersMuted)
		g.Get("/users/:userId/subscribers", app.v1UsersSubscribers)
		g.Get("/users/:userId/remixers", app.v1UsersRemixers)
		g.Get("/users/:userId/remixers/count", app.v1UsersRemixersCount)
		g.Get("/users/:userId/purchasers", app.v1UserPurchasers)
		g.Get("/users/:userId/purchasers/count", app.v1UserPurchasersCount)
		g.Get("/users/:userId/recommended-tracks", app.v1UsersRecommendedTracks)
		g.Get("/users/:userId/now-playing", app.v1UsersNowPlaying)
		g.Get("/users/:userId/coins", app.v1UsersCoins)
		g.Get("/users/:userId/coins/:mint", app.v1UsersCoin)
		g.Get("/wallet/:walletId/coins", app.v1WalletCoins)
		g.Get("/users/:userId/authorized_apps", app.v1UsersAuthorizedApps)
		g.Get("/users/:userId/authorized-apps", app.v1UsersAuthorizedApps)
		g.Get("/users/:userId/developer_apps", app.v1UsersDeveloperApps)
		g.Get("/users/:userId/developer-apps", app.v1UsersDeveloperApps)
		g.Get("/users/:userId/withdrawals/download", app.requireAuthForUserId, app.v1UsersWithdrawalsDownloadCsv)
		g.Get("/users/:userId/withdrawals/download/json", app.requireAuthForUserId, app.v1UsersWithdrawalsDownloadJson)
		g.Post("/users/:userId/follow", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UserFollow)
		g.Delete("/users/:userId/follow", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1UserFollow)
		g.Post("/users/:userId/subscribe", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UserSubscribe)
		g.Delete("/users/:userId/subscribe", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1UserSubscribe)
		g.Post("/users/:userId/mute", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UserMute)
		g.Delete("/users/:userId/mute", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1UserMute)
		g.Put("/users/:userId", app.requireAuthMiddleware, app.requireWriteScope, app.putV1User)
		g.Post("/users/:userId/notifications/campaigns/:campaignId/open", app.requireAuthMiddleware, app.requireAuthForUserId, app.v1NotificationCampaignPushOpen)

		// Tracks
		g.Get("/tracks", app.v1Tracks)
		g.Post("/tracks", app.requireAuthMiddleware, app.requireWriteScope, app.postV1Tracks)
		g.Get("/tracks/search", app.v1TracksSearch)
		g.Get("/tracks/unclaimed_id", app.v1TracksUnclaimedId)

		g.Get("/tracks/latest", app.v1TracksLatest)
			g.Get("/tracks/trending", app.v1TracksTrending)
		g.Get("/tracks/trending/ids", app.v1TracksTrendingIds)
		g.Get("/tracks/trending/winners", app.v1TracksTrendingWinners)
		g.Get("/tracks/trending/underground", app.v1TracksTrendingUnderground)
		g.Get("/tracks/trending/underground/winners", app.v1TracksTrendingUndergroundWinners)
		g.Get("/tracks/recommended", app.v1TracksTrending)
		g.Get("/tracks/recent-premium", app.v1TracksRecentPremium)
		g.Get("/tracks/usdc-purchase", app.v1TracksUsdcPurchase)
		g.Get("/tracks/inspect", app.v1TracksInspect)
		g.Get("/tracks/feeling-lucky", app.v1TracksFeelingLucky)
		g.Get("/tracks/recent-comments", app.v1TracksRecentComments)
		g.Get("/tracks/most-shared", app.v1TracksMostShared)
		g.Get("/tracks/download_counts", app.v1TracksDownloadCounts)

		g.Use("/tracks/:trackId", app.requireTrackIdMiddleware)
		g.Get("/tracks/:trackId", app.v1Track)
		g.Get("/tracks/:trackId/download_count", app.v1TrackDownloadCount)
		g.Get("/tracks/:trackId/stream", app.v1TrackStream)
		g.Get("/tracks/:trackId/download", app.v1TrackDownload)
		g.Get("/tracks/:trackId/inspect", app.v1TrackInspect)
		g.Get("/tracks/:trackId/access-info", app.v1TrackAccessInfo)
		g.Get("/tracks/:trackId/remixes", app.v1TrackRemixes)
		g.Get("/tracks/:trackId/reposts", app.v1TrackReposts)
		g.Post("/tracks/:trackId/reposts", app.requireAuthMiddleware, app.requireWriteScope, app.postV1TrackRepost)
		g.Delete("/tracks/:trackId/reposts", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1TrackRepost)
		g.Get("/tracks/:trackId/stems", app.v1TrackStems)
		g.Get("/tracks/:trackId/favorites", app.v1TrackFavorites)
		g.Post("/tracks/:trackId/favorites", app.requireAuthMiddleware, app.requireWriteScope, app.postV1TrackFavorite)
		g.Delete("/tracks/:trackId/favorites", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1TrackFavorite)
		g.Post("/tracks/:trackId/shares", app.requireAuthMiddleware, app.requireWriteScope, app.postV1TrackShare)
		g.Post("/tracks/:trackId/downloads", app.requireAuthMiddleware, app.requireWriteScope, app.postV1TrackDownload)
		g.Put("/tracks/:trackId", app.requireAuthMiddleware, app.requireWriteScope, app.putV1Track)
		g.Delete("/tracks/:trackId", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1Track)
		g.Get("/fan_club/feed", app.v1FanClubFeed)
		g.Get("/fan-club/feed", app.v1FanClubFeed)

		g.Get("/events/:eventId/comments", app.v1EventComments)
		g.Get("/events/:eventId/followers", app.v1EventsFollowers)
		g.Get("/events/:eventId/follow_state", app.v1EventFollowState)
		g.Get("/events/:eventId/follow-state", app.v1EventFollowState)
		g.Post("/events/:eventId/follow", app.requireAuthMiddleware, app.requireWriteScope, app.postV1EventFollow)
		g.Delete("/events/:eventId/follow", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1EventFollow)

		g.Get("/tracks/:trackId/comments", app.v1TrackComments)
		g.Get("/tracks/:trackId/comment_count", app.v1TrackCommentCount)
		g.Get("/tracks/:trackId/comment-count", app.v1TrackCommentCount)
		g.Get("/tracks/:trackId/comment_notification_setting", app.v1TrackCommentNotificationSetting)
		g.Get("/tracks/:trackId/comment-notification-setting", app.v1TrackCommentNotificationSetting)
		g.Get("/tracks/:trackId/remixing", app.v1TrackRemixing)
		g.Get("/tracks/:trackId/subsequent", app.v1TrackSubsequent)
		g.Get("/tracks/:trackId/top_listeners", app.v1TrackTopListeners)
		g.Get("/tracks/:trackId/top-listeners", app.v1TrackTopListeners)

		// Playlists
		g.Get("/playlists", app.v1Playlists)
		g.Post("/playlists", app.requireAuthMiddleware, app.requireWriteScope, app.postV1Playlists)
		g.Get("/playlists/search", app.v1PlaylistsSearch)
		g.Get("/playlists/unclaimed_id", app.v1PlaylistsUnclaimedId)
		g.Get("/playlists/unclaimed-id", app.v1PlaylistsUnclaimedId)
		g.Get("/playlists/trending", app.v1PlaylistsTrending)
		g.Get("/playlists/top", app.v1PlaylistsTop)
		g.Get("/playlists/new-releases", app.v1PlaylistsNewReleases)
		g.Get("/playlists/by_permalink/:handle/:slug", app.v1PlaylistByPermalink)
		g.Get("/playlists/by-permalink/:handle/:slug", app.v1PlaylistByPermalink)

		g.Use("/playlists/:playlistId", app.requirePlaylistIdMiddleware)
		g.Get("/playlists/:playlistId", app.v1Playlist)
		g.Get("/playlists/:playlistId/stream", app.v1PlaylistStream)
		g.Get("/playlists/:playlistId/reposts", app.v1PlaylistReposts)
		g.Post("/playlists/:playlistId/reposts", app.requireAuthMiddleware, app.requireWriteScope, app.postV1PlaylistRepost)
		g.Delete("/playlists/:playlistId/reposts", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1PlaylistRepost)
		g.Get("/playlists/:playlistId/favorites", app.v1PlaylistFavorites)
		g.Post("/playlists/:playlistId/favorites", app.requireAuthMiddleware, app.requireWriteScope, app.postV1PlaylistFavorite)
		g.Delete("/playlists/:playlistId/favorites", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1PlaylistFavorite)
		g.Post("/playlists/:playlistId/shares", app.requireAuthMiddleware, app.requireWriteScope, app.postV1PlaylistShare)
		g.Put("/playlists/:playlistId", app.requireAuthMiddleware, app.requireWriteScope, app.putV1Playlist)
		g.Delete("/playlists/:playlistId", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1Playlist)
		g.Get("/playlists/:playlistId/tracks", app.v1PlaylistTracks)

		// Explore
		g.Get("/explore/best-selling", app.v1ExploreBestSelling)

		// Search
		g.Get("/search/autocomplete", app.v1SearchFull)
		g.Get("/search/full", app.v1SearchFull)
		g.Get("/search/tags", app.v1SearchFull)

		// Developer Apps
		g.Get("/developer_apps/:address", app.v1DeveloperApps)
		g.Get("/developer-apps/:address", app.v1DeveloperApps)
		g.Post("/developer_apps", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UsersDeveloperApp)
		g.Post("/developer-apps", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UsersDeveloperApp)
		g.Put("/developer_apps/:address", app.requireAuthMiddleware, app.requireWriteScope, app.putV1UsersDeveloperApp)
		g.Put("/developer-apps/:address", app.requireAuthMiddleware, app.requireWriteScope, app.putV1UsersDeveloperApp)
		g.Delete("/developer_apps/:address", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1UsersDeveloperApp)
		g.Delete("/developer-apps/:address", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1UsersDeveloperApp)
		g.Post("/developer_apps/:address/access-keys/deactivate", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UsersDeveloperAppAccessKeyDeactivate)
		g.Post("/developer-apps/:address/access-keys/deactivate", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UsersDeveloperAppAccessKeyDeactivate)
		g.Post("/developer_apps/:address/register-api-key", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UsersDeveloperAppRegisterApiKey)
		g.Post("/developer-apps/:address/register-api-key", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UsersDeveloperAppRegisterApiKey)
		g.Post("/developer_apps/:address/access-keys", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UsersDeveloperAppAccessKey)
		g.Post("/developer-apps/:address/access-keys", app.requireAuthMiddleware, app.requireWriteScope, app.postV1UsersDeveloperAppAccessKey)

		// OAuth2 PKCE
		g.Get("/oauth/authorize", app.v1OAuthAuthorizeRedirect)
		g.Post("/oauth/authorize", app.v1OAuthAuthorize)
		g.Post("/oauth/token", app.v1OAuthToken)
		g.Post("/oauth/revoke", app.v1OAuthRevoke)
		g.Get("/me", app.requireAuthMiddleware, app.v1Me)
		// Legacy alias for @audius/sdk <= 14, which calls /v1/oauth/me and
		// expects the DecodedUserToken shape (userId, name, handle, ...).
		g.Get("/oauth/me", app.requireAuthMiddleware, app.v1OAuthMe)

		// Rewards
		g.Post("/rewards/claim", app.v1ClaimRewards)
		g.Post("/rewards/code", app.v1CreateRewardCode)

		// Prizes
		g.Get("/prizes", app.v1Prizes)
		g.Post("/prizes/claim", app.v1PrizesClaim)
		g.Get("/wallet/:wallet/prizes", app.v1WalletPrizes)

		// Resolve
		g.Get("/resolve", app.v1Resolve)

		// Comments
		g.Get("/comments/unclaimed_id", app.v1CommentsUnclaimedId)
		g.Get("/comments/unclaimed-id", app.v1CommentsUnclaimedId)
		g.Post("/comments", app.requireAuthMiddleware, app.requireWriteScope, app.postV1Comment)
		g.Get("/comments/:commentId", app.v1Comment)
		g.Put("/comments/:commentId", app.requireAuthMiddleware, app.requireWriteScope, app.putV1Comment)
		g.Delete("/comments/:commentId", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1Comment)
		g.Post("/comments/:commentId/react", app.requireAuthMiddleware, app.requireWriteScope, app.postV1CommentReact)
		g.Delete("/comments/:commentId/react", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1CommentReact)
		g.Post("/comments/:commentId/pin", app.requireAuthMiddleware, app.requireWriteScope, app.postV1CommentPin)
		g.Delete("/comments/:commentId/pin", app.requireAuthMiddleware, app.requireWriteScope, app.deleteV1CommentPin)
		g.Post("/comments/:commentId/report", app.requireAuthMiddleware, app.requireWriteScope, app.postV1CommentReport)

		// Tips
		g.Get("/tips", app.v1Tips)

		// Events
		g.Get("/events/unclaimed_id", app.v1EventsUnclaimedId)
		g.Get("/events/unclaimed-id", app.v1EventsUnclaimedId)
		g.Get("/events/remix-contests", app.v1EventsRemixContests)
		g.Get("/events", app.v1Events)
		g.Get("/events/all", app.v1Events)
		g.Get("/events/entity", app.v1Events)

		// Challenges
		g.Get("/challenges/undisbursed", app.v1ChallengesUndisbursed)
		g.Get("/challenges/undisbursed/:userId", app.v1ChallengesUndisbursed)
		g.Get("/challenges/disbursements", app.v1ChallengesDisbursements)
		g.Get("/challenges/:challengeId/info", app.v1ChallengesInfo)

		// Metrics
		g.Get("/metrics/genres", app.v1MetricsGenres)
		g.Get("/metrics/plays", app.v1MetricsPlays)
		g.Get("/metrics/total_plays", app.v1MetricsTotalPlays)
		g.Get("/metrics/total_artists", app.v1MetricsTotalArtists)
		g.Get("/metrics/total_wallets", app.v1MetricsTotalWallets)
		g.Get("/metrics/aggregates/apps/:time_range", app.v1MetricsApps)
		g.Get("/metrics/aggregates/apps/:time_range/unique", app.v1MetricsAppsUnique)
		g.Get("/metrics/aggregates/routes/:time_range", app.v1MetricsRoutes)
		g.Get("/metrics/aggregates/routes/trailing/:time_range", app.v1MetricsRoutesTrailing)

		// Notifications
		g.Get("/notifications/:userId", app.requireUserIdMiddleware, app.v1Notifications)
		g.Get("/notifications/:userId/playlist_updates", app.requireUserIdMiddleware, app.v1NotificationsPlaylistUpdates)
		g.Get("/notifications/campaigns/:campaignId/opens", app.v1NotificationCampaignPushOpenMetrics)

		// Protocol dashboard
		g.Get("/dashboard_wallet_users", app.v1DashboardWalletUsers)

		// Artist coins
		g.Get("/coins", app.v1Coins)
		g.Get("/coins/volume-leaders", app.v1CoinsVolumeLeaders)
		g.Get("/coins/:mint", app.v1Coin)
		g.Get("/coins/ticker/:ticker", app.v1CoinByTicker)
		g.Get("/coins/:mint/insights", app.v1CoinInsights)
		g.Get("/coins/:mint/members", app.v1CoinsMembers)
		g.Get("/coins/:mint/members/count", app.v1CoinMembersCount)
		g.Get("/coins/:mint/redeem", app.v1CoinsRedeem)
		g.Get("/coins/:mint/redeem/:code", app.v1CoinsRedeemCode)
		g.Post("/coins/:mint/redeem", app.requireAuthMiddleware, app.v1CoinsPostRedeem)
		g.Post("/coins/:mint/redeem/:code", app.requireAuthMiddleware, app.v1CoinsPostRedeem)
		g.Post("/coins", app.v1CreateCoin)
		g.Post("/coins/:mint", app.v1UpdateCoin)

		// Rendezvous and Validators
		g.Get("/rendezvous/:cid", app.v1Rendezvous)
		g.Get("/validators", app.v1Validators)
	}

	// Relay
	app.Post("/relay", app.relay)
	app.Get("/relay/decode/:encodedABI", app.decodeABI)

	// Comms
	comms := app.Group("/comms")
	// Cached/non-cached are the same as there are no other nodes to query anymore
	comms.Get("/pubkey/:userId", app.requireUserIdMiddleware, app.getPubkey)
	comms.Get("/pubkey/:userId/cached", app.requireUserIdMiddleware, app.getPubkey)

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
	comms.Get("/chats/ws", app.validateWebsocketMiddleware, websocket.New(app.getChatWebsocket))

	comms.Get("/chats/:chatId/messages", app.getChatMessages)
	comms.Get("/chats/:chatId", app.getChat)

	comms.Get("/blasts", app.getNewBlasts)

	comms.Post("/mutate", app.mutateChat)

	// These are stubbed in case they are called and will log warnings
	comms.Get("/rpc/bulk", app.getRpcBulkStub)
	comms.Post("/rpc/receive", app.postRpcReceiveStub)

	// Block confirmation
	app.Get("/block_confirmation", app.BlockConfirmation)
	app.Get("/block-confirmation", app.BlockConfirmation)

	// Health check
	app.Get("/health_check", app.healthCheck)

	// Solana health
	app.Get("/solana/health", app.solanaHealth)

	// Nodes
	app.Get("/validators", app.v1Validators)
	app.Get("/discovery", app.discoveryNodes)
	app.Get("/discovery-nodes", app.discoveryNodes)
	app.Get("/discovery/verbose", app.discoveryNodes)
	app.Get("/discovery-nodes/verbose", app.discoveryNodes)
	app.Get("/content", app.contentNodes)
	app.Get("/content-nodes", app.contentNodes)
	app.Get("/content/verbose", app.contentNodes)
	app.Get("/content-nodes/verbose", app.contentNodes)

	// Plans React app - serve static assets first, then SPA routing
	app.Static("/plans/assets", "./static/plans/dist/assets")
	app.Get("/plans/favicon.ico", func(c *fiber.Ctx) error {
		return c.SendFile("./static/plans/dist/favicon.ico")
	})
	app.Get("/plans/*", app.servePlans)

	app.Static("/", "./static")

	// Disable swagger in test environments, because it will slow things down a lot
	if config.Env != "test" {
		// Create Swagger middleware for v1
		//
		// Swagger will be available at: /v1
		// Note: v1/full endpoints exist for backwards compatibility but are not documented or exposed via SDK.
		app.Get("/v1", func(c *fiber.Ctx) error {
			return c.SendFile("./static/swagger-ui/index.html")
		})
		app.Get("/v1/swagger-oauth-callback", func(c *fiber.Ctx) error {
			return c.SendFile("./static/swagger-ui/oauth-callback.html")
		})
		app.Get("/v1/swagger.yaml", func(c *fiber.Ctx) error {
			c.Set("Content-Type", "application/yaml")
			c.Set("Cache-Control", "public, max-age=3600")
			return c.Send(swaggerV1)
		})
	}

	// gracefully handle 404
	// (this won't get hit so long as above proxy is in place)
	app.Use(func(c *fiber.Ctx) error {
		return fiber.ErrNotFound
	})

	return app
}

type BirdeyeClient interface {
	GetTokenOverview(ctx context.Context, mint string, frames string) (*birdeye.TokenOverview, error)
	GetTokenAllTimeStats(ctx context.Context, mint string) (*birdeye.TokenAllTimeStats, error)
	GetPrices(ctx context.Context, mints []string) (birdeye.TokenPriceMap, error)
}

type ApiServer struct {
	*fiber.App
	config                  *config.Config
	commsRpcProcessor       *comms.RPCProcessor
	pool                    *dbv1.DBPools
	writePool               *pgxpool.Pool
	queries                 *dbv1.Queries
	esClient                *elasticsearch.Client
	logger                  *zap.Logger
	started                 time.Time
	resolveHandleCache      *otter.Cache[string, int32]
	resolveGrantCache       *otter.Cache[string, bool]
	resolveWalletCache      *otter.Cache[string, int]
	apiAccessKeySignerCache *otter.Cache[string, apiAccessKeySignerEntry]
	oauthTokenCache         *otter.Cache[string, oauthTokenCacheEntry]
	qualifiedPlaylistsCache *otter.Cache[string, []int32]
	relatedUsersCache       *otter.Cache[string, []int32]
	requestValidator        *RequestValidator
	rewardManagerClient     *reward_manager.RewardManagerClient
	claimableTokensClient   *claimable_tokens.ClaimableTokensClient
	rewardAttester          *rewards.RewardAttester
	transactionSender       *spl.TransactionSender
	solanaConfig            *config.SolanaConfig
	antiAbuseOracles        []string
	env                     string
	openAudioSDK            *sdk.OpenAudioSDK
	audiusAppUrl            string
	skipAuthCheck           bool // set to true in a test if you don't care about auth middleware
	metricsCollector        *MetricsCollector
	rateLimitMiddleware     *RateLimitMiddleware
	birdeyeClient           BirdeyeClient
	solanaRpcClient         *rpc.Client
	meteoraDbcClient        *meteora_dbc.Client
	validators              *Nodes
	openAudioPool           *OpenAudioPool
}

func (app *ApiServer) home(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"env":     app.config.Env,
		"git":     app.config.Git,
		"started": app.started,
		"uptime":  time.Since(app.started).Truncate(time.Second).String(),
		"data": []string{
			"https://api.audius.co",
		},
	})
}

func (app *ApiServer) servePlans(c *fiber.Ctx) error {
	// Serve index.html for SPA routing (all /plans/* routes that aren't assets)
	return c.SendFile("./static/plans/dist/index.html")
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

func queryMulti(c *fiber.Ctx, key string) []string {
	var values []string
	for _, v := range c.Request().URI().QueryArgs().PeekMulti(key) {
		values = append(values, string(v))
	}
	return values
}

func (app *ApiServer) Serve() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	flushTicker := time.NewTicker(time.Second * 15)
	go func() {
		for range flushTicker.C {
			app.logger.Sync()
		}
	}()

	// Graceful shutdown
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		app.commsRpcProcessor.Shutdown()
		flushTicker.Stop()

		// Shutdown metrics collector if it exists
		if app.metricsCollector != nil {
			app.metricsCollector.Shutdown()
		}

		// Shutdown HLL aggregator if it exists
		// Removed hllAggregator

		app.Shutdown()
		app.pool.Close()
		if app.writePool != nil {
			app.writePool.Close()
		}
		app.logger.Sync()
	}()

	go func() {
		app.nodesPoller(ctx)
		app.logger.Info("Started validators poller")
	}()

	// Bind to both ipv4 and ipv6
	listener, err := net.Listen("tcp", "[::]:1323")
	if err != nil {
		app.logger.Fatal("Failed to create listener", zap.Error(err))
	}

	if err := app.Listener(listener); err != nil && err != http.ErrServerClosed {
		app.logger.Fatal("Failed to start server", zap.Error(err))
	}
}

// Move this to a new module if we add custom validation
func (app *ApiServer) ParseAndValidateQueryParams(c *fiber.Ctx, v any) error {
	if err := c.QueryParser(v); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	defaults.SetDefaults(v)
	return app.requestValidator.Validate(v)
}

func (app *ApiServer) ParseAndValidateBody(c *fiber.Ctx, v any) error {
	if err := c.BodyParser(v); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	defaults.SetDefaults(v)
	return app.requestValidator.Validate(v)
}
