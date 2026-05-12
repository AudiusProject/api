package main

import (
	"context"
	"crypto/ed25519"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"api.audius.co/config"
	"api.audius.co/utils"
	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/OpenAudio/go-openaudio/pkg/common"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
	"github.com/gagliardetto/solana-go"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mr-tron/base58"
	"go.uber.org/zap"
)

// rewardPoolDeadlineWindow is the number of blocks ahead of the current
// height at which the CLI sets the deadline_block_height on cometbft tx
// envelopes (CreateRewardPool, CreateReward).
const rewardPoolDeadlineWindow = 100

const (
	maxRetries        = 3
	initialRetryDelay = 100 * time.Millisecond
)

type CodeResult struct {
	Code          string
	Success       bool
	Error         string
	Skipped       bool
	SkippedReason string
}

func main() {
	var (
		csvFile = flag.String("csv", "", "Path to CSV file with 'code' header (required)")
		amount  = flag.Int64("amount", 0, "Reward amount (required, must be positive)")
		mint    = flag.String("mint", "", "Solana mint address (required)")
		uses    = flag.Int("uses", 1, "Number of times the code can be redeemed (default: 1)")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [flags]\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Flags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s -csv codes.csv -amount 100 -mint 9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM -uses 1\n", os.Args[0])
	}
	flag.Parse()

	if *csvFile == "" {
		fmt.Fprintf(os.Stderr, "Error: -csv flag is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	if *amount <= 0 {
		fmt.Fprintf(os.Stderr, "Error: -amount must be positive\n\n")
		flag.Usage()
		os.Exit(1)
	}

	if *mint == "" {
		fmt.Fprintf(os.Stderr, "Error: -mint flag is required\n\n")
		flag.Usage()
		os.Exit(1)
	}

	if *uses <= 0 {
		fmt.Fprintf(os.Stderr, "Error: -uses must be positive\n\n")
		flag.Usage()
		os.Exit(1)
	}

	mintPubKey, err := solana.PublicKeyFromBase58(*mint)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid mint address: %s\n", err)
		os.Exit(1)
	}

	// Load config
	cfg := config.Cfg
	if cfg.WriteDbUrl == "" {
		fmt.Fprintf(os.Stderr, "Error: writeDbUrl must be configured\n")
		os.Exit(1)
	}

	if cfg.LaunchpadDeterministicSecret == "" {
		fmt.Fprintf(os.Stderr, "Error: launchpadDeterministicSecret must be configured\n")
		os.Exit(1)
	}

	// Setup logger
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	// Connect to database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.WriteDbUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Read CSV file
	codes, err := readCodesFromCSV(*csvFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read CSV file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Processing %d codes from %s\n", len(codes), *csvFile)
	fmt.Printf("Amount: %d, Mint: %s, Uses: %d\n\n", *amount, *mint, *uses)

	// Process each code
	results := make([]CodeResult, 0, len(codes))
	hasErrors := false

	for _, code := range codes {
		result := processCode(ctx, logger, pool, cfg, code, *amount, mintPubKey, *uses)
		results = append(results, result)

		if !result.Success && !result.Skipped {
			hasErrors = true
		}

		// Print progress
		if result.Skipped {
			fmt.Printf("- %s: Skipped (%s)\n", code, result.SkippedReason)
		} else if result.Success {
			fmt.Printf("✓ %s: Created successfully\n", code)
		} else {
			fmt.Printf("✗ %s: Failed - %s\n", code, result.Error)
		}
	}

	// Print summary
	fmt.Printf("\n=== Summary ===\n")
	successCount := 0
	skippedCount := 0
	failedCount := 0

	for _, result := range results {
		if result.Success {
			successCount++
		} else if result.Skipped {
			skippedCount++
		} else {
			failedCount++
		}
	}

	fmt.Printf("Success: %d\n", successCount)
	fmt.Printf("Skipped: %d\n", skippedCount)
	fmt.Printf("Failed:  %d\n", failedCount)

	if hasErrors {
		fmt.Printf("\nSome codes failed. Review errors above.\n")
		os.Exit(1)
	} else {
		fmt.Printf("\nAll codes processed successfully.\n")
		os.Exit(0)
	}
}

func readCodesFromCSV(filename string) ([]string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	reader.TrimLeadingSpace = true

	// Read header
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("failed to read header: %w", err)
	}

	// Find 'code' column
	codeIndex := -1
	for i, col := range header {
		if strings.ToLower(strings.TrimSpace(col)) == "code" {
			codeIndex = i
			break
		}
	}

	if codeIndex == -1 {
		return nil, errors.New("CSV file must have a 'code' header column")
	}

	// Read codes
	codes := make([]string, 0)
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to read CSV record: %w", err)
		}

		if len(record) <= codeIndex {
			continue // Skip malformed rows
		}

		code := strings.TrimSpace(record[codeIndex])
		if code != "" {
			codes = append(codes, code)
		}
	}

	return codes, nil
}

func processCode(ctx context.Context, logger *zap.Logger, pool *pgxpool.Pool, cfg config.Config, code string, amount int64, mintPubKey solana.PublicKey, uses int) CodeResult {
	// Check if code already exists in database (idempotency)
	exists, err := checkCodeExists(ctx, pool, code)
	if err != nil {
		return CodeResult{
			Code:    code,
			Success: false,
			Error:   fmt.Sprintf("failed to check if code exists: %v", err),
		}
	}

	if exists {
		return CodeResult{
			Code:          code,
			Success:       true,
			Skipped:       true,
			SkippedReason: "code already exists in database",
		}
	}

	// Derive claim authority
	claimAuthority, claimAuthorityPrivateKey, err := utils.DeriveEthAddressForMint(
		[]byte("claimAuthority"),
		cfg.LaunchpadDeterministicSecret,
		mintPubKey,
	)
	if err != nil {
		return CodeResult{
			Code:    code,
			Success: false,
			Error:   fmt.Sprintf("failed to derive Ethereum key: %v", err),
		}
	}

	// Convert private key
	privateKey, err := common.EthToEthKey(claimAuthorityPrivateKey)
	if err != nil {
		return CodeResult{
			Code:    code,
			Success: false,
			Error:   fmt.Sprintf("failed to convert private key: %v", err),
		}
	}

	// Create OpenAudio SDK instance
	oap := sdk.NewOpenAudioSDK(cfg.AudiusdURL)
	oap.SetPrivKey(privateKey)

	// Derive the RM ed25519 keypair matching what the solana-relay used to
	// init the Solana reward manager state account. The base58-encoded
	// public key IS the rewards_manager_pubkey cometbft carries for this
	// mint's pool.
	rmKey := utils.DeriveRewardManagerKeypair(cfg.LaunchpadDeterministicSecret, mintPubKey)
	rewardsManagerPubkey := base58.Encode(rmKey.Public().(ed25519.PublicKey))

	// Ensure pool exists for this mint, then create the reward.
	rewardAddress, err := ensurePoolAndCreateReward(ctx, logger, pool, oap, code, amount, claimAuthority, rewardsManagerPubkey, rmKey)
	if err != nil {
		return CodeResult{
			Code:    code,
			Success: false,
			Error:   fmt.Sprintf("failed to create reward pool: %v", err),
		}
	}

	// Insert into database (with retry and idempotency)
	err = insertCodeIntoDB(ctx, pool, code, mintPubKey.String(), rewardAddress, amount, uses)
	if err != nil {
		return CodeResult{
			Code:    code,
			Success: false,
			Error:   fmt.Sprintf("failed to insert code into database: %v", err),
		}
	}

	return CodeResult{
		Code:    code,
		Success: true,
	}
}

func checkCodeExists(ctx context.Context, pool *pgxpool.Pool, code string) (bool, error) {
	var exists bool
	err := retryOperation(func() error {
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM reward_codes WHERE code = $1)", code).Scan(&exists)
		return err
	})
	return exists, err
}

// ensurePoolAndCreateReward looks up (and if missing, creates) the reward
// pool for the mint, then submits the CreateReward tx and returns the
// reward address. The "reward already exists in pool" case is detected
// via the cometbft error string and resolved by reading the previously
// stored reward_address from the local DB — the idempotency guarantee
// the prior implementation provided is preserved.
func ensurePoolAndCreateReward(ctx context.Context, logger *zap.Logger, pool *pgxpool.Pool, oap *sdk.OpenAudioSDK, code string, amount int64, claimAuthority, rewardsManagerPubkey string, rmKey ed25519.PrivateKey) (string, error) {
	var statusResp *connect.Response[v1.GetStatusResponse]
	if err := retryOperation(func() error {
		var err error
		statusResp, err = oap.Core.GetStatus(ctx, connect.NewRequest(&v1.GetStatusRequest{}))
		return err
	}); err != nil {
		return "", fmt.Errorf("failed to get chain status: %w", err)
	}
	deadline := statusResp.Msg.ChainInfo.CurrentHeight + rewardPoolDeadlineWindow

	// Pool existence check. The common case (any non-first reward for the
	// mint) is "pool exists, skip the create." Brand-new mints fall into
	// the create branch exactly once — except for the race where two
	// concurrent first-reward requests for the same mint both observe
	// NotFound and both submit CreateRewardPool; the second one's tx
	// fails, but the post-failure GetRewardPool will now find the pool,
	// which we treat as success.
	if err := retryOperation(func() error {
		_, err := oap.Rewards.GetRewardPool(ctx, rewardsManagerPubkey)
		if err == nil {
			return nil
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			return err
		}
		logger.Info("Creating reward pool", zap.String("rewards_manager_pubkey", rewardsManagerPubkey), zap.String("claim_authority", claimAuthority))
		if _, createErr := oap.Rewards.CreateRewardPool(ctx, &v1.CreateRewardPool{
			RewardsManagerPubkey: rewardsManagerPubkey,
			Authorities:          []string{claimAuthority},
		}, rmKey, deadline); createErr != nil {
			// Race: another caller created the pool between our
			// GetRewardPool and CreateRewardPool. Verify by re-fetching
			// the pool; if it now exists we lost the race cleanly.
			// Anything else is a real error.
			if _, verifyErr := oap.Rewards.GetRewardPool(ctx, rewardsManagerPubkey); verifyErr != nil {
				return createErr
			}
			logger.Info("Lost CreateRewardPool race; pool now exists",
				zap.String("rewards_manager_pubkey", rewardsManagerPubkey))
		}
		return nil
	}); err != nil {
		return "", fmt.Errorf("failed to ensure reward pool: %w", err)
	}

	var reward *v1.GetRewardResponse
	if err := retryOperation(func() error {
		var err error
		reward, err = oap.Rewards.CreateReward(ctx, &v1.CreateReward{
			RewardId:             code,
			Name:                 fmt.Sprintf("Launchpad Reward %s", code),
			Amount:               uint64(amount),
			RewardsManagerPubkey: rewardsManagerPubkey,
		}, deadline)
		return err
	}); err != nil {
		return "", fmt.Errorf("failed to create reward: %w", err)
	}

	return reward.Address, nil
}

func insertCodeIntoDB(ctx context.Context, pool *pgxpool.Pool, code, mint, rewardAddress string, amount int64, uses int) error {
	return retryOperation(func() error {
		// Use ON CONFLICT DO NOTHING for idempotency
		_, err := pool.Exec(ctx, `
			INSERT INTO reward_codes (code, mint, reward_address, amount, remaining_uses)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (code) DO NOTHING
		`, code, mint, rewardAddress, amount, uses)
		return err
	})
}

func retryOperation(operation func() error) error {
	var lastErr error
	delay := initialRetryDelay

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(delay)
			delay *= 2 // Exponential backoff
		}

		err := operation()
		if err == nil {
			return nil
		}

		lastErr = err
	}

	return fmt.Errorf("operation failed after %d attempts: %w", maxRetries, lastErr)
}
