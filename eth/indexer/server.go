package indexer

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/mcuadros/go-defaults"
	"go.uber.org/zap"
)

// healthHandlerTimeout caps how long /eth/health is willing to wait on
// downstream work (DB, etc.) before returning an error. The endpoint is
// supposed to be O(1); a 2s ceiling is generous and stops a future slow
// query from hanging the handler indefinitely.
const healthHandlerTimeout = 2 * time.Second

type Server struct {
	*fiber.App
	logger *zap.Logger
}

func NewServer(indexer *EthIndexer) *Server {
	logger := indexer.logger.Named("Server")
	app := fiber.New(fiber.Config{
		JSONEncoder:  json.Marshal,
		JSONDecoder:  json.Unmarshal,
		UnescapePath: true,
	})

	app.Get("/eth/health", func(c *fiber.Ctx) error {
		type queryParams struct {
			// Errors-out if no Transfer event has been observed in this many
			// seconds. 0 disables the check. AUDIO Transfer cadence on
			// mainnet is bursty, so default generously.
			MaxEventLagSecs int64 `query:"max_event_lag_secs" default:"600"`
		}
		var q queryParams
		if err := c.QueryParser(&q); err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		defaults.SetDefaults(&q)

		ctx, cancel := context.WithTimeout(c.Context(), healthHandlerTimeout)
		defer cancel()
		health, err := indexer.GetHealth(ctx, q.MaxEventLagSecs)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": err.Error(),
			})
		}
		if len(health.Errors) > 0 {
			c.Status(fiber.StatusInternalServerError)
		}
		return c.JSON(fiber.Map{
			"data": health,
		})
	})
	return &Server{
		App:    app,
		logger: logger,
	}
}

func (s *Server) Start(ctx context.Context) {
	go func() {
		<-ctx.Done()
		s.logger.Info("received shutdown signal, stopping server")
		if err := s.App.Shutdown(); err != nil {
			s.logger.Error("failed to shutdown app", zap.Error(err))
		}
		s.logger.Sync()
	}()

	// Bind to both ipv4 and ipv6
	listener, err := net.Listen("tcp", "[::]:1325")
	if err != nil {
		s.logger.Fatal("Failed to create listener", zap.Error(err))
	}

	if err := s.App.Listener(listener); err != nil && err != http.ErrServerClosed {
		s.logger.Fatal("Failed to start server", zap.Error(err))
	}
}
