package api

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/trashid"
	adapter "github.com/axiomhq/axiom-go/adapters/zap"
	"github.com/axiomhq/axiom-go/axiom"
	"github.com/gofiber/contrib/fiberzap/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5/pgxpool"
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

	pool, err := pgxpool.New(context.Background(), config.DbUrl)
	if err != nil {
		logger.Error("db connect failed", zap.Error(err))
	}

	app := &ApiServer{
		fiber.New(fiber.Config{
			JSONEncoder:  json.Marshal,
			JSONDecoder:  json.Unmarshal,
			ErrorHandler: errorHandler(logger),
		}),
		pool,
		dbv1.New(pool),
		logger,
		time.Now(),
	}

	app.Use(recover.New())
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

	v1 := app.Group("/v1")
	v1Full := app.Group("/v1/full")

	for _, g := range []fiber.Router{v1, v1Full} {
		// Users
		g.Get("/users", app.v1Users)

		g.Get("/users/account/:wallet", app.v1UsersAccount)

		g.Use("/users/handle/:handle", app.requireHandleMiddleware)
		g.Get("/users/handle/:handle", app.v1User)
		g.Get("/users/handle/:handle/tracks", app.v1UserTracks)
		g.Get("/users/handle/:handle/reposts", app.v1UsersReposts)

		g.Use("/users/:userId", app.requireUserIdMiddleware)
		g.Get("/users/:userId", app.v1User)
		g.Get("/users/:userId/followers", app.v1UsersFollowers)
		g.Get("/users/:userId/following", app.v1UsersFollowing)
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

		g.Get("/tracks/trending", app.v1Trending)

		g.Use("/tracks/:trackId", app.requireTrackIdMiddleware)
		g.Get("/tracks/:trackId", app.v1Track)
		g.Get("/tracks/:trackId/reposts", app.v1TracksReposts)
		g.Get("/tracks/:trackId/favorites", app.v1TracksFavorites)

		// Playlists
		g.Get("/playlists", app.v1playlists)

		g.Use("/playlists/:playlistId", app.requirePlaylistIdMiddleware)
		g.Get("/playlists/:playlistId", app.v1Playlist)
		g.Get("/playlists/:playlistId/reposts", app.v1PlaylistsReposts)
		g.Get("/playlists/:playlistId/favorites", app.v1PlaylistsFavorites)

		// Developer Apps
		g.Get("/developer_apps/:address", app.v1DeveloperApps)
	}

	app.Static("/", "./static")

	// proxy unhandled requests thru to existing discovery API
	{
		upstreams := []string{
			"https://discoveryprovider.audius.co",
			"https://discoveryprovider2.audius.co",
			"https://discoveryprovider3.audius.co",
		}
		if os.Getenv("ENV") == "stage" {
			upstreams = []string{
				"https://discoveryprovider.staging.audius.co",
				"https://discoveryprovider2.staging.audius.co",
				"https://discoveryprovider3.staging.audius.co",
				"https://discoveryprovider5.staging.audius.co",
			}
		}

		app.Use(BalancerForward(upstreams))
	}

	// gracefully handle 404
	// (this won't get hit so long as above proxy is in place)
	app.Use(func(c *fiber.Ctx) error {
		return sendError(c, 404, "Route not found")
	})

	return app
}

type ApiServer struct {
	*fiber.App
	pool    *pgxpool.Pool
	queries *dbv1.Queries
	logger  *zap.Logger
	started time.Time
}

func (app *ApiServer) home(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
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

func (app *ApiServer) resolveUserHandleToId(handle string) (int32, error) {
	// todo: can do some in memory cache here
	var userId int32
	sql := `select user_id from users where handle_lc = lower($1)`
	err := app.pool.QueryRow(context.Background(), sql, handle).Scan(&userId)
	return userId, err
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
