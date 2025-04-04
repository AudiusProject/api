package api

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	adapter "github.com/axiomhq/axiom-go/adapters/zap"
	"github.com/axiomhq/axiom-go/axiom"
	"github.com/gofiber/contrib/fiberzap/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/proxy"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Config struct {
	DbUrl        string
	AxiomToken   string
	AxiomDataset string
}

func InitLogger(config Config) *zap.Logger {
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

func NewApiServer(config Config) *ApiServer {
	logger := InitLogger(config)

	pool, err := pgxpool.New(context.Background(), config.DbUrl)
	if err != nil {
		logger.Error("db connect failed", zap.Error(err))
	}

	app := &ApiServer{
		fiber.New(fiber.Config{
			ErrorHandler: errorHandler(logger),
		}),
		pool,
		dbv1.New(pool),
		logger,
	}

	app.Use(recover.New())
	app.Use(cors.New())

	app.Use(fiberzap.New(fiberzap.Config{
		Logger: logger,
	}))

	app.Get("/", app.home)

	// v1/full
	app.Get("/v1/full/users", app.v1Users)
	app.Get("/v1/full/users/:userId/followers", app.v1UsersFollowers)
	app.Get("/v1/full/users/:userId/following", app.v1UsersFollowing)
	app.Get("/v1/full/users/:userId/mutuals", app.v1UsersMutuals)
	app.Get("/v1/full/users/:userId/supporting", app.v1UsersSupporting)

	app.Get("/v1/full/tracks", app.v1Tracks)
	app.Get("/v1/full/tracks/:trackId/reposts", app.v1TrackReposts)
	app.Get("/v1/full/tracks/:trackId/favorites", app.v1TrackFavorites)

	app.Get("/v1/full/playlists", app.v1playlists)
	app.Get("/v1/full/playlists/:playlistId/reposts", app.v1PlaylistsReposts)
	app.Get("/v1/full/playlists/:playlistId/favorites", app.v1PlaylistsFavorites)

	app.Get("/v1/full/developer_apps/:address", app.v1DeveloperApps)

	// v1 - using the same handlers but with automatic conversion via middleware
	app.Get("/v1/users", app.v1Users)
	app.Get("/v1/users/:userId/followers", app.v1UsersFollowers)
	app.Get("/v1/users/:userId/following", app.v1UsersFollowing)
	app.Get("/v1/users/:userId/mutuals", app.v1UsersMutuals)
	app.Get("/v1/users/:userId/supporting", app.v1UsersSupporting)

	app.Get("/v1/tracks", app.v1Tracks)
	app.Get("/v1/tracks/:trackId/reposts", app.v1TrackReposts)
	app.Get("/v1/tracks/:trackId/favorites", app.v1TrackFavorites)

	app.Get("/v1/developer_apps/:address", app.v1DeveloperApps)

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
		app.Use(proxy.BalancerForward(upstreams))
	}

	// gracefully handle 404
	// (this won't get hit so long as above proxy is in place)
	app.Use(func(c *fiber.Ctx) error {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{
			"code":  http.StatusNotFound,
			"error": "Route not found",
		})
	})

	return app
}

type ApiServer struct {
	*fiber.App
	pool    *pgxpool.Pool
	queries *dbv1.Queries
	logger  *zap.Logger
}

func (app *ApiServer) home(c *fiber.Ctx) error {
	return c.SendString("OK")
}

// JSON is a helper function that automatically applies conversion for v1 (non-full) routes
// If converter is nil, no conversion happens
func JSON[T any, R any](c *fiber.Ctx, data interface{}, converter func(T) R) error {
	if converter != nil && strings.HasPrefix(c.Path(), "/v1/") && !strings.HasPrefix(c.Path(), "/v1/full/") {
		if m, ok := data.(fiber.Map); ok {
			if dataField, exists := m["data"]; exists {
				if items, ok := dataField.([]T); ok {
					converted := make([]R, len(items))
					for i, item := range items {
						converted[i] = converter(item)
					}
					m["data"] = converted
				} else if item, ok := dataField.(T); ok {
					m["data"] = converter(item)
				}
			}
		}
	}

	return c.JSON(data)
}

func errorHandler(logger *zap.Logger) func(*fiber.Ctx, error) error {
	return func(ctx *fiber.Ctx, err error) error {
		code := http.StatusInternalServerError
		if err == pgx.ErrNoRows {
			code = http.StatusNotFound
		}

		if code > 499 {
			logger.Error(err.Error(),
				zap.String("url", ctx.OriginalURL()))
		}

		return ctx.Status(code).JSON(&fiber.Map{
			"code":  code,
			"error": err.Error(),
		})
	}
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
