package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
	_ "time/tzdata" // embed IANA tzdata so time.LoadLocation works on minimal images

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
	rpcClient         reclaimRentRPCClient
	transactionSender reclaimRentTransactionSender
	mints             []solana.PublicKey
	logger            *zap.Logger

	mutex     sync.Mutex
	isRunning bool
}

type reclaimRentRPCClient interface {
	GetMultipleAccountsWithOpts(
		context.Context,
		[]solana.PublicKey,
		*rpc.GetMultipleAccountsOpts,
	) (*rpc.GetMultipleAccountsResult, error)
}

type reclaimRentTransactionSender interface {
	GetFeePayer() (*solana.Wallet, error)
	SendTransactionWithRetries(
		context.Context,
		*solana.TransactionBuilder,
		rpc.CommitmentType,
		rpc.TransactionOpts,
	) (*solana.Signature, error)
}

func NewReclaimRentJob(cfg config.Config, pool database.DbPool) *ReclaimRentJob {
	logger := logging.NewZapLogger(cfg).Named("ReclaimRentJob")

	var rpcClient reclaimRentRPCClient
	if len(cfg.SolanaConfig.RpcProviders) > 0 && cfg.SolanaConfig.RpcProviders[0] != "" {
		rpcClient = rpc.New(cfg.SolanaConfig.RpcProviders[0])
	}

	var transactionSender reclaimRentTransactionSender
	if rpcClient != nil {
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

// ScheduleDailyAt runs the job once per day at hour:minute in the given
// location, starting at the next occurrence after the call.
func (j *ReclaimRentJob) ScheduleDailyAt(ctx context.Context, hour, minute int, location *time.Location) *ReclaimRentJob {
	go func() {
		for {
			now := time.Now().In(location)
			next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, location)
			if !next.After(now) {
				next = next.Add(24 * time.Hour)
			}
			j.logger.Info("Next run scheduled", zap.Time("at", next))
			timer := time.NewTimer(time.Until(next))
			select {
			case <-timer.C:
				j.Run(ctx)
			case <-ctx.Done():
				timer.Stop()
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
// destination required by the claimable-tokens program. Ensures only one
// instance runs at a time.
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

	var runErrors []error
	for _, mint := range j.mints {
		if mint.IsZero() {
			continue
		}
		if err := j.processMint(ctx, mint); err != nil {
			j.logger.Error("failed to process mint",
				zap.String("mint", mint.String()),
				zap.Error(err),
			)
			runErrors = append(runErrors, fmt.Errorf("process mint %s: %w", mint, err))
		}
	}
	return errors.Join(runErrors...)
}

type reclaimRentAccount struct {
	Account         string `db:"account"`
	EthereumAddress string `db:"ethereum_address"`
}

type reclaimRentFilterStats struct {
	MissingAccount      int
	InvalidOwner        int
	InvalidData         int
	MintMismatch        int
	NonZeroBalance      int
	InvalidEthAddress   int
	AddressMismatch     int
	WrongCloseAuthority int
}

func (s *reclaimRentFilterStats) add(other reclaimRentFilterStats) {
	s.MissingAccount += other.MissingAccount
	s.InvalidOwner += other.InvalidOwner
	s.InvalidData += other.InvalidData
	s.MintMismatch += other.MintMismatch
	s.NonZeroBalance += other.NonZeroBalance
	s.InvalidEthAddress += other.InvalidEthAddress
	s.AddressMismatch += other.AddressMismatch
	s.WrongCloseAuthority += other.WrongCloseAuthority
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
	totalCandidates := 0
	totalClosable := 0
	totalClosed := 0
	totalFailed := 0
	var filterStats reclaimRentFilterStats
	var firstProcessError error
	processErrorCount := 0
	recordProcessError := func(err error) {
		processErrorCount++
		if firstProcessError == nil {
			firstProcessError = err
		}
	}
	for {
		accounts, err := j.fetchCandidates(ctx, mint.String(), cutoff, reclaimRentDbPageSize, offset)
		if err != nil {
			return fmt.Errorf("failed to fetch candidates: %w", err)
		}
		if len(accounts) == 0 {
			break
		}
		offset += len(accounts)
		totalCandidates += len(accounts)

		filtered, pageFilterStats, err := j.filterOnChain(ctx, accounts, mint, authority)
		if err != nil {
			logger.Error("filterOnChain failed", zap.Error(err))
			recordProcessError(err)
			continue
		}
		filterStats.add(pageFilterStats)
		totalClosable += len(filtered)

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
				totalFailed += len(batch)
				recordProcessError(err)
				continue
			}
			if sig == nil {
				err := errors.New("transaction sender returned a nil signature")
				logger.Error("processBatch failed",
					zap.Error(err),
					zap.Int("batch_size", len(batch)),
				)
				totalFailed += len(batch)
				recordProcessError(err)
				continue
			}
			logger.Info("Reclaimed batch",
				zap.String("signature", sig.String()),
				zap.Int("accounts", len(batch)),
			)
			totalClosed += len(batch)
		}
	}
	logger.Info("Done processing mint",
		zap.Int("db_candidates", totalCandidates),
		zap.Int("onchain_closable", totalClosable),
		zap.Int("total_closed", totalClosed),
		zap.Int("total_failed", totalFailed),
		zap.Int("processing_errors", processErrorCount),
		zap.Int("skipped_missing", filterStats.MissingAccount),
		zap.Int("skipped_invalid_owner", filterStats.InvalidOwner),
		zap.Int("skipped_invalid_data", filterStats.InvalidData),
		zap.Int("skipped_mint_mismatch", filterStats.MintMismatch),
		zap.Int("skipped_nonzero_balance", filterStats.NonZeroBalance),
		zap.Int("skipped_invalid_eth_address", filterStats.InvalidEthAddress),
		zap.Int("skipped_address_mismatch", filterStats.AddressMismatch),
		zap.Int("skipped_wrong_close_authority", filterStats.WrongCloseAuthority),
	)
	if firstProcessError != nil {
		return fmt.Errorf(
			"%d processing operation(s) failed (%d close attempts affected): %w",
			processErrorCount,
			totalFailed,
			firstProcessError,
		)
	}
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

func (j *ReclaimRentJob) filterOnChain(
	ctx context.Context,
	batch []reclaimRentAccount,
	mint solana.PublicKey,
	authority solana.PublicKey,
) ([]reclaimRentAccount, reclaimRentFilterStats, error) {
	var stats reclaimRentFilterStats
	pubkeys := make([]solana.PublicKey, 0, len(batch))
	for _, acct := range batch {
		pubkey, err := solana.PublicKeyFromBase58(acct.Account)
		if err != nil {
			return nil, stats, fmt.Errorf("invalid account public key %q: %w", acct.Account, err)
		}
		pubkeys = append(pubkeys, pubkey)
	}
	res, err := j.rpcClient.GetMultipleAccountsWithOpts(ctx, pubkeys, &rpc.GetMultipleAccountsOpts{
		Encoding: solana.EncodingBase64,
	})
	if err != nil {
		return nil, stats, fmt.Errorf("failed to get accounts: %w", err)
	}
	if len(res.Value) != len(batch) {
		return nil, stats, fmt.Errorf("RPC returned %d accounts for a batch of %d", len(res.Value), len(batch))
	}

	filtered := make([]reclaimRentAccount, 0, len(batch))
	for i, info := range res.Value {
		if info == nil {
			stats.MissingAccount++
			continue
		}
		if !info.Owner.Equals(solana.TokenProgramID) {
			stats.InvalidOwner++
			continue
		}
		if info.Data == nil {
			stats.InvalidData++
			continue
		}
		var ta token.Account
		if err := bin.NewBorshDecoder(info.Data.GetBinary()).Decode(&ta); err != nil {
			stats.InvalidData++
			continue
		}
		if !ta.Mint.Equals(mint) {
			stats.MintMismatch++
			continue
		}
		if ta.Amount != 0 {
			stats.NonZeroBalance++
			continue
		}
		if !common.IsHexAddress(batch[i].EthereumAddress) {
			stats.InvalidEthAddress++
			continue
		}
		expectedUserBank, err := claimable_tokens.DeriveUserBankAccount(
			mint,
			common.HexToAddress(batch[i].EthereumAddress),
		)
		if err != nil {
			return nil, stats, fmt.Errorf("derive user bank for %q: %w", batch[i].Account, err)
		}
		if !pubkeys[i].Equals(expectedUserBank) {
			stats.AddressMismatch++
			continue
		}

		closeAuthority := ta.Owner
		if ta.CloseAuthority != nil {
			closeAuthority = *ta.CloseAuthority
		}
		if !closeAuthority.Equals(authority) {
			stats.WrongCloseAuthority++
			continue
		}
		filtered = append(filtered, batch[i])
	}
	return filtered, stats, nil
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
	rentDestination := solana.MustPublicKeyFromBase58(claimable_tokens.DefaultRentDestinationAddress)
	for _, acct := range batch {
		userBank, err := solana.PublicKeyFromBase58(acct.Account)
		if err != nil {
			return nil, fmt.Errorf("invalid account public key %q: %w", acct.Account, err)
		}
		if !common.IsHexAddress(acct.EthereumAddress) {
			return nil, fmt.Errorf("invalid Ethereum address %q for account %s", acct.EthereumAddress, acct.Account)
		}
		inst := claimable_tokens.NewCloseInstructionBuilder().
			SetUserBank(userBank).
			SetAuthority(authority).
			SetDestination(rentDestination).
			SetEthAddress(common.HexToAddress(acct.EthereumAddress))
		builder.AddInstruction(inst.Build())
	}

	return j.transactionSender.SendTransactionWithRetries(ctx, builder, rpc.CommitmentConfirmed, rpc.TransactionOpts{})
}
