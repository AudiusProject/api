package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"api.audius.co/config"
	"api.audius.co/database"
	"api.audius.co/logging"
	"api.audius.co/solana/spl"
	"api.audius.co/solana/spl/programs/claimable_tokens"
	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/programs/token"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

const (
	reclaimRentLookback   = 7 * 24 * time.Hour
	reclaimRentBatchSize  = 15
	reclaimRentDbPageSize = 1000
)

type ReclaimRentJob struct {
	cfg               config.Config
	pool              database.DbPool
	rpcClient         *rpc.Client
	transactionSender *spl.TransactionSender
	mints             []solana.PublicKey
	logger            *zap.Logger

	mutex     sync.Mutex
	isRunning bool
}

func NewReclaimRentJob(cfg config.Config, pool database.DbPool) *ReclaimRentJob {
	logger := logging.NewZapLogger(cfg).Named("ReclaimRentJob")

	var rpcClient *rpc.Client
	if len(cfg.SolanaConfig.RpcProviders) > 0 {
		rpcClient = rpc.New(cfg.SolanaConfig.RpcProviders[0])
	}

	var transactionSender *spl.TransactionSender
	if len(cfg.SolanaConfig.RpcProviders) > 0 {
		transactionSender = spl.NewTransactionSender(cfg.SolanaConfig.FeePayers, cfg.SolanaConfig.RpcProviders)
	}

	return &ReclaimRentJob{
		cfg:               cfg,
		pool:              pool,
		rpcClient:         rpcClient,
		transactionSender: transactionSender,
		mints: []solana.PublicKey{
			cfg.SolanaConfig.MintAudio,
			cfg.SolanaConfig.MintUSDC,
		},
		logger: logger,
	}
}

// ScheduleEvery runs the job every `duration` until the context is cancelled.
func (j *ReclaimRentJob) ScheduleEvery(ctx context.Context, duration time.Duration) *ReclaimRentJob {
	go func() {
		ticker := time.NewTicker(duration)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				j.Run(ctx)
			case <-ctx.Done():
				j.logger.Info("Job schedule shutting down")
				return
			}
		}
	}()
	return j
}

// Run executes the job once
func (j *ReclaimRentJob) Run(ctx context.Context) {
	j.logger.Info("Job started")
	if err := j.run(ctx); err != nil {
		j.logger.Error("Job run failed", zap.Error(err))
	} else {
		j.logger.Info("Job completed successfully")
	}
}

// Closes zero-balance claimable token accounts created in the last 7 days
// for the configured AUDIO and USDC mints, returning the rent lamports to the
// fee payer that signs each transaction. Ensures only one instance runs at a time.
func (j *ReclaimRentJob) run(ctx context.Context) error {
	j.mutex.Lock()
	if j.isRunning {
		j.mutex.Unlock()
		return fmt.Errorf("job is already running")
	}
	j.isRunning = true
	j.mutex.Unlock()
	defer func() {
		j.mutex.Lock()
		j.isRunning = false
		j.mutex.Unlock()
	}()

	if j.rpcClient == nil || j.transactionSender == nil {
		j.logger.Warn("No Solana RPC configured, skipping reclaim_rent")
		return nil
	}
	if len(j.cfg.SolanaConfig.FeePayers) == 0 {
		j.logger.Warn("No Solana fee payers configured, skipping reclaim_rent")
		return nil
	}

	for _, mint := range j.mints {
		if mint.IsZero() {
			continue
		}
		if err := j.processMint(ctx, mint); err != nil {
			j.logger.Error("failed to process mint",
				zap.String("mint", mint.String()),
				zap.Error(err),
			)
		}
	}
	return nil
}

type reclaimRentAccount struct {
	Account         string `db:"account"`
	EthereumAddress string `db:"ethereum_address"`
}

func (j *ReclaimRentJob) processMint(ctx context.Context, mint solana.PublicKey) error {
	logger := j.logger.With(zap.String("mint", mint.String()))
	logger.Info("Processing mint")

	authority, _, err := claimable_tokens.DeriveAuthority(mint)
	if err != nil {
		return fmt.Errorf("failed to derive authority: %w", err)
	}

	cutoff := time.Now().Add(-reclaimRentLookback)
	offset := 0
	totalClosed := 0
	for {
		accounts, err := j.fetchCandidates(ctx, mint.String(), cutoff, reclaimRentDbPageSize, offset)
		if err != nil {
			return fmt.Errorf("failed to fetch candidates: %w", err)
		}
		if len(accounts) == 0 {
			break
		}
		offset += len(accounts)

		filtered, err := j.filterOnChain(ctx, accounts)
		if err != nil {
			logger.Error("filterOnChain failed", zap.Error(err))
			continue
		}

		for i := 0; i < len(filtered); i += reclaimRentBatchSize {
			end := i + reclaimRentBatchSize
			if end > len(filtered) {
				end = len(filtered)
			}
			batch := filtered[i:end]
			sig, err := j.processBatch(ctx, batch, authority)
			if err != nil {
				logger.Error("processBatch failed",
					zap.Error(err),
					zap.Int("batch_size", len(batch)),
				)
				continue
			}
			logger.Info("Reclaimed batch",
				zap.String("signature", sig.String()),
				zap.Int("accounts", len(batch)),
			)
			totalClosed += len(batch)
		}
	}
	logger.Info("Done processing mint", zap.Int("total_closed", totalClosed))
	return nil
}

func (j *ReclaimRentJob) fetchCandidates(ctx context.Context, mint string, since time.Time, limit, offset int) ([]reclaimRentAccount, error) {
	sql := `
		SELECT DISTINCT sca.account, sca.ethereum_address
		FROM sol_claimable_accounts sca
		JOIN sol_token_account_balances stab ON stab.account = sca.account
		WHERE sca.mint = @mint
		  AND stab.mint = @mint
		  AND stab.balance = 0
		  AND stab.created_at > @since
		ORDER BY sca.account
		LIMIT @limit OFFSET @offset
	`
	rows, err := j.pool.Query(ctx, sql, pgx.NamedArgs{
		"mint":   mint,
		"since":  since,
		"limit":  limit,
		"offset": offset,
	})
	if err != nil {
		return nil, err
	}
	return pgx.CollectRows(rows, pgx.RowToStructByName[reclaimRentAccount])
}

func (j *ReclaimRentJob) filterOnChain(ctx context.Context, batch []reclaimRentAccount) ([]reclaimRentAccount, error) {
	pubkeys := make([]solana.PublicKey, 0, len(batch))
	for _, acct := range batch {
		pubkeys = append(pubkeys, solana.MustPublicKeyFromBase58(acct.Account))
	}
	res, err := j.rpcClient.GetMultipleAccountsWithOpts(ctx, pubkeys, &rpc.GetMultipleAccountsOpts{
		Encoding: solana.EncodingBase64,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	filtered := make([]reclaimRentAccount, 0, len(batch))
	for i, info := range res.Value {
		if info == nil {
			continue
		}
		var ta token.Account
		if err := bin.NewBorshDecoder(info.Data.GetBinary()).Decode(&ta); err != nil {
			continue
		}
		if ta.Amount != 0 {
			continue
		}
		filtered = append(filtered, batch[i])
	}
	return filtered, nil
}

func (j *ReclaimRentJob) processBatch(ctx context.Context, batch []reclaimRentAccount, authority solana.PublicKey) (*solana.Signature, error) {
	if len(batch) == 0 {
		return nil, nil
	}

	payer, err := j.transactionSender.GetFeePayer()
	if err != nil {
		return nil, err
	}

	builder := solana.NewTransactionBuilder().SetFeePayer(payer.PublicKey())
	for _, acct := range batch {
		inst := claimable_tokens.NewCloseInstructionBuilder().
			SetUserBank(solana.MustPublicKeyFromBase58(acct.Account)).
			SetAuthority(authority).
			SetDestination(payer.PublicKey()).
			SetEthAddress(common.HexToAddress(acct.EthereumAddress))
		builder.AddInstruction(inst.Build())
	}

	return j.transactionSender.SendTransactionWithRetries(ctx, builder, rpc.CommitmentConfirmed, rpc.TransactionOpts{})
}
