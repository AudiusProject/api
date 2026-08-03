package indexer

import (
	"context"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"sync/atomic"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// Contract function selectors. Computed at startup from their signatures so
// the source of truth is the human-readable form, not magic bytes.
var (
	balanceOfSelector              = keccakSelector("balanceOf(address)")
	totalStakedForSelector         = keccakSelector("totalStakedFor(address)")
	getTotalDelegatorStakeSelector = keccakSelector("getTotalDelegatorStake(address)")
)

func keccakSelector(sig string) []byte {
	return crypto.Keccak256([]byte(sig))[:4]
}

// Checkpoint key for the AUDIO Transfer subscription. Stored in the
// eth_indexer_checkpoints table so we can resume after a restart.
const checkpointName = "audio_transfers"

// Backfill chunk size — Alchemy's free tier caps eth_getLogs at 10K blocks.
const backfillChunkBlocks = 9000

// Reconnect backoff bounds for the WS subscription.
const (
	reconnectInitialBackoff = 1 * time.Second
	reconnectMaxBackoff     = 60 * time.Second
)

// Stale-refresh defaults. The sweep complements the live WS subscription:
// it picks the K oldest rows in eth_wallet_balances by updated_at, re-reads
// their balances, and upserts. This recovers from drift, missed events
// during disconnects, and multi-wallet user backfills where we couldn't
// decompose user_balances.associated_wallets_balance per-wallet.
//
// Default cadence: 50 wallets / 30s ≈ 1.7 wallets/sec ≈ 144K/day. With
// ~3.15M tracked wallets a full sweep takes ~22 days. Tune via the env
// vars below.
const (
	ethStaleRefreshDefaultInterval  = 30 * time.Second
	ethStaleRefreshDefaultBatchSize = 50
)

type EthIndexer struct {
	config          config.Config
	pool            database.DbPool
	logger          *zap.Logger
	httpURL         string
	wsURL           string
	audioContract   common.Address
	stakingContract common.Address
	delegateManager common.Address
	transferTopic   common.Hash

	httpClient *ethclient.Client

	staleRefreshInterval  time.Duration
	staleRefreshBatchSize int

	// State surfaced via /eth/health
	connected     atomic.Bool
	lastBlockSeen atomic.Uint64
	lastEventAt   atomic.Pointer[time.Time]
}

// envIntDefault reads an env var as int, falling back to def on missing,
// empty, or unparseable values.
func envIntDefault(name string, def int) int {
	if s := os.Getenv(name); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func New(cfg config.Config) *EthIndexer {
	logger := logging.NewZapLogger(cfg).Named("EthIndexer")

	connConfig, err := pgxpool.ParseConfig(cfg.WriteDbUrl)
	if err != nil {
		panic(fmt.Errorf("error parsing database URL: %w", err))
	}
	pool, err := pgxpool.NewWithConfig(context.Background(), connConfig)
	if err != nil {
		panic(fmt.Errorf("error connecting to database: %w", err))
	}

	return &EthIndexer{
		config:                cfg,
		pool:                  pool,
		logger:                logger,
		httpURL:               cfg.EthRpcUrl,
		wsURL:                 cfg.EthWsUrl,
		audioContract:         common.HexToAddress(cfg.EthAudioContractAddress),
		stakingContract:       common.HexToAddress(cfg.EthStakingContractAddress),
		delegateManager:       common.HexToAddress(cfg.EthDelegateManagerContractAddress),
		transferTopic:         crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")),
		staleRefreshInterval:  time.Duration(envIntDefault("ethStaleRefreshIntervalSecs", int(ethStaleRefreshDefaultInterval.Seconds()))) * time.Second,
		staleRefreshBatchSize: envIntDefault("ethStaleRefreshBatchSize", ethStaleRefreshDefaultBatchSize),
	}
}

func (e *EthIndexer) Start(ctx context.Context) error {
	if e.httpURL == "" || e.wsURL == "" {
		e.logger.Warn("ethRpcUrl/ethWsUrl not configured, indexer is a no-op",
			zap.String("ethRpcUrl", e.httpURL),
			zap.String("ethWsUrl", e.wsURL),
		)
		<-ctx.Done()
		return nil
	}

	httpClient, err := ethclient.DialContext(ctx, e.httpURL)
	if err != nil {
		return fmt.Errorf("dialing http rpc: %w", err)
	}
	e.httpClient = httpClient
	defer httpClient.Close()

	go e.ScheduleStaleRefresh(ctx)

	e.runSubscriptionLoop(ctx)
	return nil
}

// runSubscriptionLoop opens the WS subscription, processes the live stream,
// and reconnects with exponential backoff on failure. On every (re)connect it
// first backfills the gap between the stored checkpoint and the current head.
func (e *EthIndexer) runSubscriptionLoop(ctx context.Context) {
	backoff := reconnectInitialBackoff
	for {
		if err := ctx.Err(); err != nil {
			return
		}

		err := e.runOnce(ctx)
		if err == nil {
			// Context cancelled — graceful exit.
			return
		}
		e.connected.Store(false)
		e.logger.Error("subscription loop ended, will reconnect",
			zap.Error(err),
			zap.Duration("backoff", backoff),
		)

		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff *= 2
		if backoff > reconnectMaxBackoff {
			backoff = reconnectMaxBackoff
		}
	}
}

func (e *EthIndexer) runOnce(ctx context.Context) error {
	wsClient, err := ethclient.DialContext(ctx, e.wsURL)
	if err != nil {
		return fmt.Errorf("dialing ws rpc: %w", err)
	}
	defer wsClient.Close()

	// Backfill any blocks we missed since the last checkpoint.
	if err := e.backfill(ctx); err != nil {
		return fmt.Errorf("backfill: %w", err)
	}

	query := ethereum.FilterQuery{
		Addresses: []common.Address{e.audioContract},
		Topics:    [][]common.Hash{{e.transferTopic}},
	}
	logCh := make(chan types.Log, 256)
	sub, err := wsClient.SubscribeFilterLogs(ctx, query, logCh)
	if err != nil {
		return fmt.Errorf("subscribe filter logs: %w", err)
	}
	defer sub.Unsubscribe()
	e.connected.Store(true)
	e.logger.Info("subscription established",
		zap.String("contract", e.audioContract.Hex()),
		zap.String("topic", e.transferTopic.Hex()),
	)

	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-sub.Err():
			return fmt.Errorf("subscription error: %w", err)
		case lg := <-logCh:
			e.handleLog(ctx, lg)
		}
	}
}

// backfill walks from the last-checkpointed block up to current head in
// chunks, processing any Transfer events that touch tracked wallets.
func (e *EthIndexer) backfill(ctx context.Context) error {
	currentBlock, err := e.httpClient.BlockNumber(ctx)
	if err != nil {
		return fmt.Errorf("getting current block: %w", err)
	}

	startBlock, err := e.loadCheckpoint(ctx)
	if err != nil {
		return fmt.Errorf("loading checkpoint: %w", err)
	}
	if startBlock == 0 {
		// First boot — no point replaying chain history. Start from current
		// head; we'll learn balances via the live subscription and an
		// on-demand catch-up sweep can fill in stragglers later.
		startBlock = currentBlock
	}
	if startBlock >= currentBlock {
		return nil
	}

	e.logger.Info("backfilling missed blocks",
		zap.Uint64("from", startBlock+1),
		zap.Uint64("to", currentBlock),
	)

	for from := startBlock + 1; from <= currentBlock; from += backfillChunkBlocks {
		to := from + backfillChunkBlocks - 1
		if to > currentBlock {
			to = currentBlock
		}
		query := ethereum.FilterQuery{
			FromBlock: new(big.Int).SetUint64(from),
			ToBlock:   new(big.Int).SetUint64(to),
			Addresses: []common.Address{e.audioContract},
			Topics:    [][]common.Hash{{e.transferTopic}},
		}
		logs, err := e.httpClient.FilterLogs(ctx, query)
		if err != nil {
			return fmt.Errorf("FilterLogs [%d, %d]: %w", from, to, err)
		}
		e.processLogs(ctx, logs)
		if err := e.saveCheckpoint(ctx, to); err != nil {
			return fmt.Errorf("saving checkpoint: %w", err)
		}
	}
	return nil
}

// handleLog is the live-stream entry point: one event at a time.
func (e *EthIndexer) handleLog(ctx context.Context, lg types.Log) {
	e.processLogs(ctx, []types.Log{lg})
	if err := e.saveCheckpoint(ctx, lg.BlockNumber); err != nil {
		e.logger.Warn("failed to save checkpoint", zap.Error(err))
	}
}

// processLogs extracts from/to addresses from each Transfer event, filters
// to only those that are tracked wallets, fetches their current balanceOf
// via fan-out, and upserts.
func (e *EthIndexer) processLogs(ctx context.Context, logs []types.Log) {
	if len(logs) == 0 {
		return
	}
	now := time.Now()
	e.lastEventAt.Store(&now)

	// Collect candidate addresses from the events.
	candidates := make(map[common.Address]uint64, len(logs)*2)
	for _, lg := range logs {
		if len(lg.Topics) < 3 {
			continue
		}
		from := common.HexToAddress(lg.Topics[1].Hex())
		to := common.HexToAddress(lg.Topics[2].Hex())
		if blk, ok := candidates[from]; !ok || lg.BlockNumber > blk {
			candidates[from] = lg.BlockNumber
		}
		if blk, ok := candidates[to]; !ok || lg.BlockNumber > blk {
			candidates[to] = lg.BlockNumber
		}
		if lg.BlockNumber > e.lastBlockSeen.Load() {
			e.lastBlockSeen.Store(lg.BlockNumber)
		}
	}

	// Filter to only tracked addresses.
	tracked, err := e.filterTracked(ctx, candidates)
	if err != nil {
		e.logger.Warn("failed to filter tracked addresses", zap.Error(err))
		return
	}
	if len(tracked) == 0 {
		return
	}

	updated := e.refreshAddresses(ctx, tracked, candidates)
	if updated > 0 {
		e.logger.Info("refreshed balances from events", zap.Int("updated", updated))
	}
}

// refreshAddresses reads balanceOf + totalStakedFor +
// getTotalDelegatorStake for each address via a single Multicall3
// `aggregate3` (chunked at multicallChunkSize holders) and upserts the
// summed results. blockByAddr is optional per-address block context: for
// live Transfer events it's the block the event was mined in; for
// stale-refresh sweeps it's nil so the existing blocknumber column is
// preserved. Returns the number of addresses upserted.
func (e *EthIndexer) refreshAddresses(ctx context.Context, addrs []common.Address, blockByAddr map[common.Address]uint64) int {
	if len(addrs) == 0 {
		return 0
	}

	balances, err := e.totalAudioBalances(ctx, addrs)
	if err != nil {
		e.logger.Warn("totalAudioBalances failed",
			zap.Int("holders", len(addrs)),
			zap.Error(err),
		)
		return 0
	}

	updates := make([]balanceUpdate, 0, len(balances))
	for _, addr := range addrs {
		bal, ok := balances[addr]
		if !ok {
			continue
		}
		block := uint64(0)
		if blockByAddr != nil {
			block = blockByAddr[addr]
		}
		updates = append(updates, balanceUpdate{addr: addr, bal: bal, block: block})
	}

	if err := e.upsertBalanceUpdates(ctx, updates); err != nil {
		e.logger.Error("failed to upsert balances", zap.Error(err))
		return 0
	}
	return len(updates)
}

func (e *EthIndexer) filterTracked(ctx context.Context, candidates map[common.Address]uint64) ([]common.Address, error) {
	if len(candidates) == 0 {
		return nil, nil
	}
	addrs := make([]string, 0, len(candidates))
	for a := range candidates {
		addrs = append(addrs, lowerHex(a))
	}
	sql := `
		SELECT DISTINCT wallet
		FROM (
			SELECT LOWER(wallet) AS wallet
			FROM users
			WHERE LOWER(wallet) = ANY(@addrs)
			UNION ALL
			SELECT LOWER(wallet) AS wallet
			FROM associated_wallets
			WHERE chain = 'eth'
				AND is_delete = FALSE
				AND LOWER(wallet) = ANY(@addrs)
		) t
	`
	rows, err := e.pool.Query(ctx, sql, pgx.NamedArgs{"addrs": addrs})
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tracked := make([]common.Address, 0)
	for rows.Next() {
		var w string
		if err := rows.Scan(&w); err != nil {
			return nil, err
		}
		tracked = append(tracked, common.HexToAddress(w))
	}
	return tracked, rows.Err()
}


type balanceUpdate struct {
	addr  common.Address
	bal   *big.Int
	block uint64
}

func (e *EthIndexer) upsertBalanceUpdates(ctx context.Context, updates []balanceUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	wallets := make([]string, 0, len(updates))
	weis := make([]string, 0, len(updates))
	blocks := make([]int64, 0, len(updates))
	for _, u := range updates {
		wallets = append(wallets, lowerHex(u.addr))
		weis = append(weis, u.bal.String())
		blocks = append(blocks, int64(u.block))
	}
	// blocknumber semantics:
	//   - new block > 0 (Transfer-event path): take GREATEST with existing
	//   - new block = 0 (stale-refresh sweep): preserve existing column,
	//     don't downgrade a real block to 0 just because we re-read latest
	_, err := e.pool.Exec(ctx, `
		INSERT INTO eth_wallet_balances (wallet, balance, blocknumber, updated_at)
		SELECT
			unnest(@wallets::text[]),
			unnest(@balances::text[])::numeric,
			NULLIF(unnest(@blocks::bigint[]), 0),
			NOW()
		ON CONFLICT (wallet) DO UPDATE SET
			balance = EXCLUDED.balance,
			blocknumber = CASE
				WHEN EXCLUDED.blocknumber IS NULL THEN eth_wallet_balances.blocknumber
				ELSE GREATEST(COALESCE(eth_wallet_balances.blocknumber, 0), EXCLUDED.blocknumber)
			END,
			updated_at = NOW()
	`, pgx.NamedArgs{
		"wallets":  wallets,
		"balances": weis,
		"blocks":   blocks,
	})
	return err
}

// ScheduleStaleRefresh runs a background sweep that re-reads the oldest
// rows in eth_wallet_balances by updated_at and upserts the fresh values.
// Complements the live WS subscription: it recovers from drift, fills in
// rows that were never touched by a Transfer event (multi-wallet backfill
// placeholders), and re-reads anything that went stale while the WS was
// disconnected. Bounded throughput by design (batchSize per tick).
func (e *EthIndexer) ScheduleStaleRefresh(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			e.logger.Error("stale-refresh goroutine panicked, will not restart",
				zap.Any("panic", r),
			)
		}
	}()

	e.logger.Info("starting stale-refresh sweep",
		zap.Duration("interval", e.staleRefreshInterval),
		zap.Int("batch_size", e.staleRefreshBatchSize),
	)
	ticker := time.NewTicker(e.staleRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.runStaleRefresh(ctx)
		}
	}
}

func (e *EthIndexer) runStaleRefresh(ctx context.Context) {
	addrs, err := e.selectStaleWallets(ctx)
	if err != nil {
		e.logger.Warn("stale refresh: select failed", zap.Error(err))
		return
	}
	if len(addrs) == 0 {
		return
	}
	updated := e.refreshAddresses(ctx, addrs, nil)
	if updated > 0 {
		e.logger.Info("stale refresh: tick complete",
			zap.Int("requested", len(addrs)),
			zap.Int("updated", updated),
		)
	}
}

// selectStaleWallets returns the K rows from eth_wallet_balances with the
// oldest updated_at. Indexed by eth_wallet_balances_updated_at_idx.
func (e *EthIndexer) selectStaleWallets(ctx context.Context) ([]common.Address, error) {
	rows, err := e.pool.Query(ctx, `
		SELECT wallet
		FROM eth_wallet_balances
		ORDER BY updated_at ASC
		LIMIT $1
	`, e.staleRefreshBatchSize)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]common.Address, 0, e.staleRefreshBatchSize)
	for rows.Next() {
		var w string
		if err := rows.Scan(&w); err != nil {
			return nil, err
		}
		out = append(out, common.HexToAddress(w))
	}
	return out, rows.Err()
}

func (e *EthIndexer) loadCheckpoint(ctx context.Context) (uint64, error) {
	var last int64
	err := e.pool.QueryRow(ctx,
		`SELECT last_block FROM eth_indexer_checkpoints WHERE name = $1`,
		checkpointName,
	).Scan(&last)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil
		}
		return 0, err
	}
	if last < 0 {
		return 0, nil
	}
	return uint64(last), nil
}

func (e *EthIndexer) saveCheckpoint(ctx context.Context, block uint64) error {
	_, err := e.pool.Exec(ctx, `
		INSERT INTO eth_indexer_checkpoints (name, last_block, updated_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (name) DO UPDATE SET
			last_block = GREATEST(eth_indexer_checkpoints.last_block, EXCLUDED.last_block),
			updated_at = NOW()
	`, checkpointName, int64(block))
	return err
}

type ethHealth struct {
	Errors          []string   `json:"errors,omitempty"`
	Connected       bool       `json:"connected"`
	RpcConfigured   bool       `json:"rpc_configured"`
	LastBlockSeen   uint64     `json:"last_block_seen"`
	CheckpointBlock uint64     `json:"checkpoint_block"`
	LastEventAt     *time.Time `json:"last_event_at"`
}

// GetHealth returns indexer liveness in O(1) — all values are either in
// memory or come from a single-row PK lookup. Wallet-population counts
// previously lived on this response but were expensive on prod
// (UNION/COUNT across users + associated_wallets ≈ 3M rows, no index, can
// take 30s+) and don't actually answer "is the indexer alive?", which is
// what a health endpoint is for. If you want population stats, query
// eth_wallet_balances directly.
func (e *EthIndexer) GetHealth(ctx context.Context, maxEventLagSecs int64) (*ethHealth, error) {
	checkpoint, err := e.loadCheckpoint(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading checkpoint: %w", err)
	}

	errs := make([]string, 0)
	if !e.connected.Load() && e.wsURL != "" {
		errs = append(errs, "websocket subscription not connected")
	}
	if e.wsURL == "" {
		errs = append(errs, "ethWsUrl is not configured")
	}
	if lastEvent := e.lastEventAt.Load(); lastEvent != nil && maxEventLagSecs > 0 {
		if since := time.Since(*lastEvent); since > time.Duration(maxEventLagSecs)*time.Second {
			errs = append(errs, fmt.Sprintf("no events seen for %s", since.Truncate(time.Second)))
		}
	}

	return &ethHealth{
		Errors:          errs,
		Connected:       e.connected.Load(),
		RpcConfigured:   e.httpURL != "" && e.wsURL != "",
		LastBlockSeen:   e.lastBlockSeen.Load(),
		CheckpointBlock: checkpoint,
		LastEventAt:     e.lastEventAt.Load(),
	}, nil
}

func (e *EthIndexer) Close() {
	e.pool.Close()
}

func lowerHex(a common.Address) string {
	// common.Address.Hex() returns checksummed; we want lowercase to match
	// how associated_wallets.wallet is stored.
	return "0x" + common.Bytes2Hex(a.Bytes())
}
