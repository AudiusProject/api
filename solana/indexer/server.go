package indexer

import (
	"context"
	"encoding/json"
	"net"
	"net/http"

	"github.com/gofiber/fiber/v2"
	"github.com/mcuadros/go-defaults"
	"go.uber.org/zap"
)

type Server struct {
	*fiber.App
	logger *zap.Logger
}

func NewServer(indexer *SolanaIndexer) *Server {
	logger := indexer.logger.Named("Server")
	app := fiber.New(fiber.Config{
		JSONEncoder:  json.Marshal,
		JSONDecoder:  json.Unmarshal,
		UnescapePath: true,
	})

	app.Get("/solana/health", func(c *fiber.Ctx) error {
		type queryParams struct {
			MaxSlotDiff   int64 `query:"max_slot_diff" default:"100"`
			MaxRetryQueue int   `query:"max_retry_queue" default:"10"`
		}
		var q queryParams
		err := c.QueryParser(&q)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": err.Error(),
			})
		}

		defaults.SetDefaults(&q)

		health, err := indexer.GetHealth(c.Context(), uint64(q.MaxSlotDiff), q.MaxRetryQueue)
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
	listener, err := net.Listen("tcp", "[::]:1324")
	if err != nil {
		s.logger.Fatal("Failed to create listener", zap.Error(err))
	}

	if err := s.App.Listener(listener); err != nil && err != http.ErrServerClosed {
		s.logger.Fatal("Failed to start server", zap.Error(err))
	}
}
