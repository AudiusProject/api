package indexer

import (
	"context"
	"fmt"
	"math/big"
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
	"golang.org/x/sync/errgroup"
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

// Refresh fan-out: how many balanceOf calls we'll issue in parallel after a
// burst of events. Keeps a burst from saturating the upstream.
const balanceFetchWorkers = 8

// Reconnect backoff bounds for the WS subscription.
const (
	reconnectInitialBackoff = 1 * time.Second
	reconnectMaxBackoff     = 60 * time.Second
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

	// State surfaced via /eth/health
	connected     atomic.Bool
	lastBlockSeen atomic.Uint64
	lastEventAt   atomic.Pointer[time.Time]
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
		config:          cfg,
		pool:            pool,
		logger:          logger,
		httpURL:         cfg.EthRpcUrl,
		wsURL:           cfg.EthWsUrl,
		audioContract:   common.HexToAddress(cfg.EthAudioContractAddress),
		stakingContract: common.HexToAddress(cfg.EthStakingContractAddress),
		delegateManager: common.HexToAddress(cfg.EthDelegateManagerContractAddress),
		transferTopic:   crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)")),
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

	// Fan out balanceOf calls.
	jobs := make(chan common.Address, len(tracked))
	results := make(chan balanceUpdate, len(tracked))
	workers := balanceFetchWorkers
	if workers > len(tracked) {
		workers = len(tracked)
	}
	for w := 0; w < workers; w++ {
		go func() {
			for addr := range jobs {
				bal, err := e.totalAudioBalance(ctx, addr)
				if err != nil {
					e.logger.Warn("totalAudioBalance failed",
						zap.String("addr", addr.Hex()),
						zap.Error(err),
					)
					results <- balanceUpdate{} // sentinel so receiver count matches
					continue
				}
				results <- balanceUpdate{addr: addr, bal: bal, block: candidates[addr]}
			}
		}()
	}
	for _, addr := range tracked {
		jobs <- addr
	}
	close(jobs)

	updates := make([]balanceUpdate, 0, len(tracked))
	for i := 0; i < len(tracked); i++ {
		select {
		case <-ctx.Done():
			return
		case r := <-results:
			if r.bal == nil {
				continue
			}
			updates = append(updates, r)
		}
	}
	if err := e.upsertBalanceUpdates(ctx, updates); err != nil {
		e.logger.Error("failed to upsert balances", zap.Error(err))
	} else if len(updates) > 0 {
		e.logger.Info("refreshed balances from events",
			zap.Int("updated", len(updates)),
		)
	}
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

// totalAudioBalance returns balanceOf + totalStakedFor + getTotalDelegatorStake,
// matching the Python discovery-provider's `associated_wallets_balance`
// computation. All three calls run in parallel; any failure fails the whole
// read (we'd rather skip the wallet this round than persist a partial total).
func (e *EthIndexer) totalAudioBalance(ctx context.Context, holder common.Address) (*big.Int, error) {
	var balance, staked, delegated *big.Int

	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		v, err := e.uintCall(gctx, e.audioContract, balanceOfSelector, holder)
		if err != nil {
			return fmt.Errorf("balanceOf: %w", err)
		}
		balance = v
		return nil
	})
	g.Go(func() error {
		v, err := e.uintCall(gctx, e.stakingContract, totalStakedForSelector, holder)
		if err != nil {
			return fmt.Errorf("totalStakedFor: %w", err)
		}
		staked = v
		return nil
	})
	g.Go(func() error {
		v, err := e.uintCall(gctx, e.delegateManager, getTotalDelegatorStakeSelector, holder)
		if err != nil {
			return fmt.Errorf("getTotalDelegatorStake: %w", err)
		}
		delegated = v
		return nil
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	sum := new(big.Int).Add(balance, staked)
	sum.Add(sum, delegated)
	return sum, nil
}

// uintCall invokes a `func(address) returns (uint256)` style getter and
// decodes the result as a big.Int.
func (e *EthIndexer) uintCall(ctx context.Context, contract common.Address, selector []byte, holder common.Address) (*big.Int, error) {
	data := append(append([]byte{}, selector...), common.LeftPadBytes(holder.Bytes(), 32)...)
	msg := ethereum.CallMsg{To: &contract, Data: data}
	out, err := e.httpClient.CallContract(ctx, msg, nil)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return big.NewInt(0), nil
	}
	return new(big.Int).SetBytes(out), nil
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
	_, err := e.pool.Exec(ctx, `
		INSERT INTO eth_wallet_balances (wallet, balance, blocknumber, updated_at)
		SELECT
			unnest(@wallets::text[]),
			unnest(@balances::text[])::numeric,
			unnest(@blocks::bigint[]),
			NOW()
		ON CONFLICT (wallet) DO UPDATE SET
			balance = EXCLUDED.balance,
			blocknumber = GREATEST(eth_wallet_balances.blocknumber, EXCLUDED.blocknumber),
			updated_at = NOW()
	`, pgx.NamedArgs{
		"wallets":  wallets,
		"balances": weis,
		"blocks":   blocks,
	})
	return err
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
	TrackedWallets  int64      `json:"tracked_wallets"`
	CachedWallets   int64      `json:"cached_wallets"`
}

func (e *EthIndexer) GetHealth(ctx context.Context, maxEventLagSecs int64) (*ethHealth, error) {
	checkpoint, err := e.loadCheckpoint(ctx)
	if err != nil {
		return nil, fmt.Errorf("loading checkpoint: %w", err)
	}

	var tracked, cached int64
	err = e.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM (
				SELECT LOWER(wallet) FROM users WHERE wallet IS NOT NULL AND wallet <> ''
				UNION
				SELECT LOWER(wallet) FROM associated_wallets WHERE chain = 'eth' AND is_delete = FALSE
			) t) AS tracked,
			(SELECT COUNT(*) FROM eth_wallet_balances) AS cached
	`).Scan(&tracked, &cached)
	if err != nil {
		return nil, fmt.Errorf("counting wallets: %w", err)
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
		TrackedWallets:  tracked,
		CachedWallets:   cached,
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
