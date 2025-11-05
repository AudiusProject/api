package indexer

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"api.audius.co/api/dbv1"
	"api.audius.co/config"
	"api.audius.co/logging"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type Server struct {
	app    *fiber.App
	pool   *dbv1.DBPools
	logger *zap.Logger
}

const MAX_SLOT_DIFF = 100
const MAX_RETRY_QUEUE = 10

func NewServer(config config.Config) *Server {
	logger := logging.NewZapLogger(config).Named("Server")

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

	solanaRpc := rpc.New(config.SolanaConfig.RpcProviders[0])

	app := fiber.New(fiber.Config{
		JSONEncoder:  json.Marshal,
		JSONDecoder:  json.Unmarshal,
		UnescapePath: true,
	})

	app.Get("/solana/health", func(c *fiber.Ctx) error {

		chainSlot, err := solanaRpc.GetSlot(c.Context(), rpc.CommitmentConfirmed)
		if err != nil {
			return fmt.Errorf("failed to get chain slot: %w", err)
		}

		sql := `
			WITH retry_queue_by_indexer AS (
				SELECT
					indexer,
					COUNT(*) AS retry_queue_count
				FROM sol_retry_queue
				GROUP BY indexer
			) SELECT DISTINCT ON (name) 
				name,
				to_slot AS indexed_slot,
				@chain_slot - to_slot AS slot_diff,
				COALESCE(retry_queue_count, 0) AS retry_queue_count,
				updated_at
			FROM sol_slot_checkpoints
			LEFT JOIN retry_queue_by_indexer ON indexer = name
			ORDER BY name, from_slot DESC
		;
		`

		rows, err := pool.Query(c.Context(), sql, pgx.NamedArgs{
			"chain_slot": chainSlot,
		})
		if err != nil {
			return fmt.Errorf("failed to query indexer health: %w", err)
		}

		type indexerHealthRow struct {
			Name            string     `json:"name"`
			SlotDiff        uint64     `json:"slot_diff"`
			IndexedSlot     uint64     `json:"indexed_slot"`
			RetryQueueCount int        `json:"retry_queue_count"`
			UpdatedAt       *time.Time `db:"updated_at"`
		}
		healths, err := pgx.CollectRows(rows, pgx.RowToStructByName[indexerHealthRow])
		if err != nil {
			return fmt.Errorf("failed to collect indexer health rows: %w", err)
		}

		type solanaHealth struct {
			ChainSlot uint64   `json:"chain_slot"`
			Errors    []string `json:"errors,omitempty"`
			Indexers  []indexerHealthRow
		}
		health := solanaHealth{
			ChainSlot: chainSlot,
			Errors:    make([]string, 0),
			Indexers:  healths,
		}

		for _, h := range health.Indexers {
			if h.RetryQueueCount > MAX_RETRY_QUEUE {
				c.Status(fiber.StatusInternalServerError)
				health.Errors = append(health.Errors, fmt.Sprintf("indexer %s has high retry queue count: %d", h.Name, h.RetryQueueCount))
			}

			if h.SlotDiff > MAX_SLOT_DIFF {
				c.Status(fiber.StatusInternalServerError)
				health.Errors = append(health.Errors, fmt.Sprintf("indexer %s has high slot diff: %d", h.Name, h.SlotDiff))
			}
		}

		return c.JSON(fiber.Map{
			"data": health,
		})
	})

	return &Server{
		app:    app,
		pool:   pool,
		logger: logger,
	}
}

func (s *Server) Start(ctx context.Context) {
	flushTicker := time.NewTicker(time.Second * 15)
	defer flushTicker.Stop()

	go func() {
		for range flushTicker.C {
			s.logger.Sync()
		}
	}()

	go func() {
		<-ctx.Done()
		s.logger.Info("received shutdown signal, stopping server")
		s.Shutdown(context.Background())
	}()

	// Bind to both ipv4 and ipv6
	listener, err := net.Listen("tcp", "[::]:1324")
	if err != nil {
		s.logger.Fatal("Failed to create listener", zap.Error(err))
	}

	if err := s.app.Listener(listener); err != nil && err != http.ErrServerClosed {
		s.logger.Fatal("Failed to start server", zap.Error(err))
	}
}

func (s *Server) Shutdown(ctx context.Context) {
	if err := s.app.Shutdown(); err != nil {
		s.logger.Error("failed to shutdown app", zap.Error(err))
	}
	s.pool.Close()
	s.logger.Sync()
}
