package api

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"strings"
	"time"

	"api.audius.co/config"
	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	corev1connect "github.com/OpenAudio/go-openaudio/pkg/api/core/v1/v1connect"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"golang.org/x/net/http2"
	"google.golang.org/protobuf/proto"
)

// NewChainFlusher reads rows from new_chain_queue and forwards them to the new
// Core chain. On startup it deletes rows covered by the backfill (confirmed_block
// < cfg.NewChainFlushFromBlock), then sends the rest in id order, one at a time.
// Sequential processing is required to preserve transaction ordering across users
// (dev apps can act on behalf of other users, creating cross-user dependencies).
type NewChainFlusher struct {
	cfg         *config.Config
	writePool   *pgxpool.Pool
	chainClient corev1connect.CoreServiceClient
	logger      *zap.Logger
}

func NewNewChainFlusher(cfg *config.Config, writePool *pgxpool.Pool, logger *zap.Logger) *NewChainFlusher {
	chainURL := cfg.NewChainURL
	if !strings.HasPrefix(chainURL, "http://") && !strings.HasPrefix(chainURL, "https://") {
		chainURL = "https://" + chainURL
	}

	var httpClient *http.Client
	if strings.HasPrefix(chainURL, "http://") {
		// h2c: plain HTTP/2, bypasses nginx and talks directly to the gRPC port
		httpClient = &http.Client{
			Transport: &http2.Transport{
				AllowHTTP: true,
				DialTLSContext: func(ctx context.Context, network, addr string, _ *tls.Config) (net.Conn, error) {
					return net.Dial(network, addr)
				},
			},
		}
	} else if cfg.NewChainInsecureSkipVerify {
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	} else {
		httpClient = http.DefaultClient
	}

	return newFlusherWithClient(cfg, writePool, corev1connect.NewCoreServiceClient(httpClient, chainURL), logger)
}

// newFlusherWithClient constructs a flusher with a pre-built chain client.
// Used in tests to inject a plain HTTP/1.1 client against httptest.Server.
func newFlusherWithClient(cfg *config.Config, writePool *pgxpool.Pool, client corev1connect.CoreServiceClient, logger *zap.Logger) *NewChainFlusher {
	return &NewChainFlusher{
		cfg:         cfg,
		writePool:   writePool,
		chainClient: client,
		logger:      logger.With(zap.String("component", "new_chain_flusher")),
	}
}

// Start trims backfill-covered rows, then continuously drains new_chain_queue.
// Runs until ctx is cancelled.
func (f *NewChainFlusher) Start(ctx context.Context) {
	if err := f.trimBackfillRows(ctx); err != nil {
		f.logger.Error("trim failed", zap.Error(err))
		// non-fatal: continue flushing with whatever rows remain
	}

	f.logger.Info("starting flush loop",
		zap.String("new_chain_url", f.cfg.NewChainURL),
	)

	const batchSize = 500

	for {
		if ctx.Err() != nil {
			return
		}

		rows, err := f.fetchBatch(ctx, batchSize)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			f.logger.Error("fetch batch failed", zap.Error(err))
			sleep(ctx, 2*time.Second)
			continue
		}

		if len(rows) == 0 {
			// Queue is empty; pause before checking again.
			sleep(ctx, 500*time.Millisecond)
			continue
		}

		for _, row := range rows {
			if ctx.Err() != nil {
				return
			}
			if err := f.flushRow(ctx, row); err != nil && ctx.Err() == nil {
				// Do not advance past a failed row: ordering requires every tx
				// to land before the next one is sent. Sleep and let the outer
				// loop re-fetch the same row.
				f.logger.Error("flush row failed, retrying", zap.Int64("id", row.id), zap.Error(err))
				sleep(ctx, 2*time.Second)
				break
			}
		}
	}
}

type queueRow struct {
	id    int64
	txRaw []byte
}

// fetchBatch returns the next rows to send, oldest first.
//
// NewChainFlushToBlock, when set, is a ceiling: rows confirmed above it are left
// pending. It is the mirror of NewChainFlushFromBlock and exists for the indexer
// cutover, where the old-chain indexer stops at a height L and everything
// confirmed at or below L must be on the new chain before the new indexer starts
// (ROLLOUT.md, Runbook step 12).
//
// A filter, not a stop. Enqueue is dispatched asynchronously, so confirmed_block
// is only roughly ordered by id — halting at the first row above the ceiling
// would strand a row below it, which would then flush after the boundary was
// recorded and be indexed twice.
//
// Rows with a NULL confirmed_block are held while the ceiling is set: they
// cannot be placed relative to L, and holding is the recoverable choice —
// they flush once the ceiling is lifted. Drain or inspect them before the
// cutover rather than discovering them during it.
func (f *NewChainFlusher) fetchBatch(ctx context.Context, limit int) ([]queueRow, error) {
	query := `SELECT id, tx_data FROM new_chain_queue ORDER BY id LIMIT $1`
	args := []any{limit}
	if f.cfg.NewChainFlushToBlock > 0 {
		query = `SELECT id, tx_data FROM new_chain_queue
		         WHERE confirmed_block IS NOT NULL AND confirmed_block <= $2
		         ORDER BY id LIMIT $1`
		args = append(args, f.cfg.NewChainFlushToBlock)
	}
	rows, err := f.writePool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []queueRow
	for rows.Next() {
		var r queueRow
		if err := rows.Scan(&r.id, &r.txRaw); err != nil {
			return nil, err
		}
		result = append(result, r)
	}
	return result, rows.Err()
}

func (f *NewChainFlusher) flushRow(ctx context.Context, row queueRow) error {
	var tx v1.ManageEntityLegacy
	if err := proto.Unmarshal(row.txRaw, &tx); err != nil {
		// Corrupt row — delete it and move on rather than retrying forever.
		f.logger.Error("corrupt queue row, deleting", zap.Int64("id", row.id), zap.Error(err))
		_, _ = f.writePool.Exec(ctx, `DELETE FROM new_chain_queue WHERE id = $1`, row.id)
		return nil
	}

	req := connect.NewRequest(&v1.ForwardTransactionRequest{
		Transaction: &v1.SignedTransaction{
			RequestId: uuid.NewString(),
			Transaction: &v1.SignedTransaction_ManageEntity{
				ManageEntity: &tx,
			},
		},
	})

	const (
		baseDelay = 500 * time.Millisecond
		maxDelay  = 15 * time.Second
	)
	delay := baseDelay
	for {
		_, err := f.chainClient.ForwardTransaction(ctx, req)
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "mempool full") {
			return err
		}
		f.logger.Warn("mempool full, pausing flush", zap.Duration("wait", delay))
		sleep(ctx, delay)
		if ctx.Err() != nil {
			return ctx.Err()
		}
		delay *= 2
		if delay > maxDelay {
			delay = maxDelay
		}
	}

	_, err := f.writePool.Exec(ctx, `DELETE FROM new_chain_queue WHERE id = $1`, row.id)
	return err
}

// trimBackfillRows deletes all queue rows whose confirmed_block is before the
// configured flush-from block, i.e. rows already covered by the genesis backfill.
// Rows with a NULL confirmed_block are kept: NULL < $1 evaluates to NULL (falsy) in SQL.
func (f *NewChainFlusher) trimBackfillRows(ctx context.Context) error {
	if f.cfg.NewChainFlushFromBlock <= 0 {
		return nil
	}
	tag, err := f.writePool.Exec(ctx,
		`DELETE FROM new_chain_queue WHERE confirmed_block < $1`,
		f.cfg.NewChainFlushFromBlock,
	)
	if err != nil {
		return err
	}
	f.logger.Info("trimmed backfill-covered rows",
		zap.Int64("deleted", tag.RowsAffected()),
		zap.Int64("flush_from_block", f.cfg.NewChainFlushFromBlock),
	)
	return nil
}

func sleep(ctx context.Context, d time.Duration) {
	select {
	case <-ctx.Done():
	case <-time.After(d):
	}
}
