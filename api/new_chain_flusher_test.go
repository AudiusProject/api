package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"connectrpc.com/connect"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	corev1connect "github.com/OpenAudio/go-openaudio/pkg/api/core/v1/v1connect"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// mockCoreService implements only ForwardTransaction; all other methods return Unimplemented.
type mockCoreService struct {
	corev1connect.UnimplementedCoreServiceHandler
	received []*corev1.ManageEntityLegacy
	calls    atomic.Int32
}

func (m *mockCoreService) ForwardTransaction(_ context.Context, req *connect.Request[corev1.ForwardTransactionRequest]) (*connect.Response[corev1.ForwardTransactionResponse], error) {
	m.calls.Add(1)
	if me := req.Msg.Transaction.GetManageEntity(); me != nil {
		m.received = append(m.received, me)
	}
	return connect.NewResponse(&corev1.ForwardTransactionResponse{}), nil
}

func newTestFlusher(t *testing.T, cfg *config.Config) (*NewChainFlusher, *mockCoreService) {
	t.Helper()
	mock := &mockCoreService{}
	mux := http.NewServeMux()
	path, handler := corev1connect.NewCoreServiceHandler(mock)
	mux.Handle(path, handler)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	pool := database.CreateTestDatabase(t, "test_api")

	// Fallback for a template DB predating the new_chain_queue migrations;
	// a current template already has the table and this is a no-op.
	_, err := pool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS new_chain_queue (
			id              bigserial PRIMARY KEY,
			created_at      timestamptz NOT NULL DEFAULT now(),
			tx_data         bytea NOT NULL,
			confirmed_block bigint,
			flushed_at      timestamptz,
			skip_reason     text
		)
	`)
	require.NoError(t, err)

	cfg.NewChainURL = srv.URL

	logger, _ := zap.NewDevelopment()
	client := corev1connect.NewCoreServiceClient(http.DefaultClient, srv.URL)
	f := newFlusherWithClient(cfg, pool, client, logger)
	return f, mock
}

func sampleTx(entityID int64) *corev1.ManageEntityLegacy {
	return &corev1.ManageEntityLegacy{
		UserId:     1,
		EntityType: "Track",
		EntityId:   entityID,
		Action:     "Create",
		Metadata:   "{}",
		Nonce:      "0xdeadbeef",
	}
}

func insertQueueRow(t *testing.T, f *NewChainFlusher, tx *corev1.ManageEntityLegacy, confirmedBlock *int64) {
	t.Helper()
	b, err := proto.Marshal(tx)
	require.NoError(t, err)
	_, err = f.writePool.Exec(context.Background(),
		`INSERT INTO new_chain_queue (tx_data, confirmed_block) VALUES ($1, $2)`,
		b, confirmedBlock,
	)
	require.NoError(t, err)
}

// pendingDepth counts rows the flusher still has to send. Rows are never
// deleted, so this is the count of unflushed rows rather than the table size.
func pendingDepth(t *testing.T, f *NewChainFlusher) int {
	t.Helper()
	var n int
	err := f.writePool.QueryRow(context.Background(),
		`SELECT count(*) FROM new_chain_queue WHERE flushed_at IS NULL`).Scan(&n)
	require.NoError(t, err)
	return n
}

// totalDepth counts every row, flushed or not.
func totalDepth(t *testing.T, f *NewChainFlusher) int {
	t.Helper()
	var n int
	err := f.writePool.QueryRow(context.Background(), `SELECT count(*) FROM new_chain_queue`).Scan(&n)
	require.NoError(t, err)
	return n
}

// skipReasons returns the skip_reason of every non-pending row, in id order,
// with NULL (i.e. genuinely forwarded) rendered as "sent".
func skipReasons(t *testing.T, f *NewChainFlusher) []string {
	t.Helper()
	rows, err := f.writePool.Query(context.Background(),
		`SELECT coalesce(skip_reason, 'sent') FROM new_chain_queue
		 WHERE flushed_at IS NOT NULL ORDER BY id`)
	require.NoError(t, err)
	defer rows.Close()
	var out []string
	for rows.Next() {
		var r string
		require.NoError(t, rows.Scan(&r))
		out = append(out, r)
	}
	require.NoError(t, rows.Err())
	return out
}

// TestEnqueueForNewChain verifies that enqueueForNewChain inserts a row with the
// correct tx_data and confirmed_block.
func TestEnqueueForNewChain(t *testing.T) {
	app := emptyTestApp(t)

	// Fallback for a template DB predating the new_chain_queue migrations.
	_, err := app.writePool.Exec(context.Background(), `
		CREATE TABLE IF NOT EXISTS new_chain_queue (
			id              bigserial PRIMARY KEY,
			created_at      timestamptz NOT NULL DEFAULT now(),
			tx_data         bytea NOT NULL,
			confirmed_block bigint,
			flushed_at      timestamptz,
			skip_reason     text
		)
	`)
	require.NoError(t, err)

	tx := sampleTx(42)
	app.enqueueForNewChain(tx, 999)

	// enqueueForNewChain is fire-and-forget; give it a moment to write.
	require.Eventually(t, func() bool {
		var n int
		app.writePool.QueryRow(context.Background(), `SELECT count(*) FROM new_chain_queue`).Scan(&n) //nolint:errcheck
		return n == 1
	}, 2*time.Second, 50*time.Millisecond)

	var txData []byte
	var confirmedBlock int64
	err = app.writePool.QueryRow(context.Background(),
		`SELECT tx_data, confirmed_block FROM new_chain_queue LIMIT 1`,
	).Scan(&txData, &confirmedBlock)
	require.NoError(t, err)
	require.Equal(t, int64(999), confirmedBlock)

	var got corev1.ManageEntityLegacy
	require.NoError(t, proto.Unmarshal(txData, &got))
	require.Equal(t, int64(42), got.EntityId)
	require.Equal(t, "Track", got.EntityType)
}

// TestNewChainFlusherTrim verifies that rows with confirmed_block < FlushFromBlock
// are marked skipped on startup rather than deleted, and rows at or above the
// threshold stay pending.
func TestNewChainFlusherTrim(t *testing.T) {
	cfg := &config.Config{NewChainFlushFromBlock: 100}
	f, _ := newTestFlusher(t, cfg)

	block50 := int64(50)
	block99 := int64(99)
	block100 := int64(100)
	block200 := int64(200)
	insertQueueRow(t, f, sampleTx(1), &block50)  // should be trimmed
	insertQueueRow(t, f, sampleTx(2), &block99)  // should be trimmed
	insertQueueRow(t, f, sampleTx(3), &block100) // kept (boundary)
	insertQueueRow(t, f, sampleTx(4), &block200) // kept
	insertQueueRow(t, f, sampleTx(5), nil)       // NULL confirmed_block — kept

	require.Equal(t, 5, pendingDepth(t, f))

	err := f.trimBackfillRows(context.Background())
	require.NoError(t, err)

	require.Equal(t, 3, pendingDepth(t, f), "two backfill-covered rows should no longer be pending")
	require.Equal(t, 5, totalDepth(t, f), "trim must not delete rows")
	require.Equal(t, []string{"backfilled", "backfilled"}, skipReasons(t, f))

	// Verify the entity IDs still pending.
	rows, err := f.writePool.Query(context.Background(),
		`SELECT (tx_data) FROM new_chain_queue WHERE flushed_at IS NULL ORDER BY id`,
	)
	require.NoError(t, err)
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var b []byte
		require.NoError(t, rows.Scan(&b))
		var me corev1.ManageEntityLegacy
		require.NoError(t, proto.Unmarshal(b, &me))
		ids = append(ids, me.EntityId)
	}
	require.Equal(t, []int64{3, 4, 5}, ids)
}

// TestNewChainFlusherSends verifies that the flusher forwards all queued rows to
// the new chain and marks them flushed — retaining them — on success.
func TestNewChainFlusherSends(t *testing.T) {
	cfg := &config.Config{} // no trim
	f, mock := newTestFlusher(t, cfg)

	block10 := int64(10)
	block20 := int64(20)
	block30 := int64(30)
	insertQueueRow(t, f, sampleTx(1), &block10)
	insertQueueRow(t, f, sampleTx(2), &block20)
	insertQueueRow(t, f, sampleTx(3), &block30)

	require.Equal(t, 3, pendingDepth(t, f))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go f.Start(ctx)

	require.Eventually(t, func() bool {
		return mock.calls.Load() == 3
	}, 5*time.Second, 50*time.Millisecond, "expected 3 ForwardTransaction calls")

	require.Eventually(t, func() bool {
		return pendingDepth(t, f) == 0
	}, 5*time.Second, 50*time.Millisecond, "expected queue to drain")

	// Forwarded rows are retained so they can be re-driven onto a rebuilt chain.
	require.Equal(t, 3, totalDepth(t, f), "flushed rows must not be deleted")
	require.Equal(t, []string{"sent", "sent", "sent"}, skipReasons(t, f))

	// Verify all three entity IDs were forwarded.
	receivedIDs := make([]int64, len(mock.received))
	for i, me := range mock.received {
		receivedIDs[i] = me.EntityId
	}
	require.ElementsMatch(t, []int64{1, 2, 3}, receivedIDs)
}

// TestNewChainFlusherTrimThenSend verifies the combined trim+flush flow:
// rows before the cutoff block are dropped; rows after are forwarded.
func TestNewChainFlusherTrimThenSend(t *testing.T) {
	cfg := &config.Config{NewChainFlushFromBlock: 50}
	f, mock := newTestFlusher(t, cfg)

	block10 := int64(10)   // pre-backfill — trimmed
	block49 := int64(49)   // pre-backfill — trimmed
	block50 := int64(50)   // post-backfill — flushed
	block100 := int64(100) // post-backfill — flushed
	insertQueueRow(t, f, sampleTx(1), &block10)
	insertQueueRow(t, f, sampleTx(2), &block49)
	insertQueueRow(t, f, sampleTx(3), &block50)
	insertQueueRow(t, f, sampleTx(4), &block100)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	go f.Start(ctx)

	require.Eventually(t, func() bool {
		return mock.calls.Load() == 2
	}, 5*time.Second, 50*time.Millisecond, "expected 2 ForwardTransaction calls (post-trim)")

	require.Eventually(t, func() bool {
		return pendingDepth(t, f) == 0
	}, 5*time.Second, 50*time.Millisecond, "expected queue to drain")

	// Two skipped by the trim, two genuinely sent — all four retained.
	require.Equal(t, 4, totalDepth(t, f))
	require.Equal(t, []string{"backfilled", "backfilled", "sent", "sent"}, skipReasons(t, f))

	receivedIDs := make([]int64, len(mock.received))
	for i, me := range mock.received {
		receivedIDs[i] = me.EntityId
	}
	require.ElementsMatch(t, []int64{3, 4}, receivedIDs)
}

// TestNewChainFlusherRedriveAfterRegeneration is the reason rows are retained
// rather than deleted. The genesis chain is regenerated before it ships, so
// everything already forwarded has to be replayable onto the rebuilt chain.
//
// Simulates that: drain the queue against chain v1, then clear flushed_at (what
// an operator does when repointing at the rebuilt chain) and confirm the same
// transactions are forwarded again. With delete-on-success there would be
// nothing left to re-drive.
func TestNewChainFlusherRedriveAfterRegeneration(t *testing.T) {
	cfg := &config.Config{}
	f, mock := newTestFlusher(t, cfg)

	block10 := int64(10)
	block20 := int64(20)
	insertQueueRow(t, f, sampleTx(1), &block10)
	insertQueueRow(t, f, sampleTx(2), &block20)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	go f.Start(ctx)

	require.Eventually(t, func() bool {
		return mock.calls.Load() == 2
	}, 5*time.Second, 50*time.Millisecond, "expected both rows forwarded to chain v1")
	require.Eventually(t, func() bool {
		return pendingDepth(t, f) == 0
	}, 5*time.Second, 50*time.Millisecond)
	cancel()

	// The rows are still here — that is the point.
	require.Equal(t, 2, totalDepth(t, f))

	// Repoint at the rebuilt chain: everything becomes pending again.
	_, err := f.writePool.Exec(context.Background(),
		`UPDATE new_chain_queue SET flushed_at = NULL, skip_reason = NULL`)
	require.NoError(t, err)
	require.Equal(t, 2, pendingDepth(t, f))

	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	go f.Start(ctx2)

	require.Eventually(t, func() bool {
		return mock.calls.Load() == 4
	}, 5*time.Second, 50*time.Millisecond, "expected both rows forwarded again after re-drive")
	require.Eventually(t, func() bool {
		return pendingDepth(t, f) == 0
	}, 5*time.Second, 50*time.Millisecond)
}
