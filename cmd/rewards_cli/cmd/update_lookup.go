package cmd

import (
	"context"
	"fmt"

	"api.audius.co/config"
	"api.audius.co/logging"
	"api.audius.co/solana/spl"
	"api.audius.co/solana/spl/programs/address_lookup_table"
	"api.audius.co/solana/spl/programs/reward_manager"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var updateLookupCmd = &cobra.Command{
	Use:   "update-lookup",
	Short: "Update the address lookup table with new sender accounts",
	Long: `Updates the reward manager's address lookup table by adding sender accounts
for any newly registered validators or anti-abuse oracles that are missing.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get flags
		rpcOverride, _ := cmd.Flags().GetString("rpc")
		openAudioURLOverride, _ := cmd.Flags().GetString("openAudioURL")
		keypairPath, _ := cmd.Flags().GetString("keypair")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		// Initialize logger
		cfg := config.Cfg
		logger := logging.NewZapLogger(cfg)

		// Initialize config based on environment
		if rpcOverride != "" {
			cfg.SolanaConfig.RpcProviders = []string{rpcOverride}
		}
		if openAudioURLOverride != "" {
			cfg.OpenAudioURLs = []string{openAudioURLOverride}
		}
		if keypairPath != "" {
			privKey, err := solana.PrivateKeyFromSolanaKeygenFile(keypairPath)
			if err != nil {
				return fmt.Errorf("failed to load keypair from %s: %w", keypairPath, err)
			}
			payer, err := solana.WalletFromPrivateKeyBase58(privKey.String())
			if err != nil {
				return fmt.Errorf("failed to create wallet from private key: %w", err)
			}
			cfg.SolanaConfig.FeePayers = []solana.Wallet{*payer}
		}

		if len(cfg.SolanaConfig.FeePayers) == 0 {
			return fmt.Errorf("no fee payer configured")
		}

		ctx := cmd.Context()
		return runUpdateLookup(ctx, cfg, dryRun, logger)
	},
}

func init() {
	updateLookupCmd.Flags().Bool("dry-run", false, "Show what would be added without making changes")
	rootCmd.AddCommand(updateLookupCmd)
}

func runUpdateLookup(ctx context.Context, cfg config.Config, dryRun bool, logger *zap.Logger) error {
	// Initialize clients
	if len(cfg.OpenAudioURLs) == 0 {
		return fmt.Errorf("no OpenAudio URLs configured")
	}
	openAudioSDK := sdk.NewOpenAudioSDK(cfg.OpenAudioURLs[0])

	if len(cfg.SolanaConfig.RpcProviders) == 0 {
		return fmt.Errorf("no Solana RPC providers configured")
	}
	rpcClient := rpc.New(cfg.SolanaConfig.RpcProviders[0])

	// Initialize reward manager
	reward_manager.SetProgramID(cfg.SolanaConfig.RewardManagerProgramID)
	rewardManagerClient, err := reward_manager.NewRewardManagerClient(
		rpcClient,
		cfg.SolanaConfig.RewardManagerProgramID,
		cfg.SolanaConfig.RewardManagerState,
		cfg.SolanaConfig.RewardManagerLookupTable,
		logger,
	)
	if err != nil {
		return fmt.Errorf("failed to initialize reward manager client: %w", err)
	}

	lookupTableAddress := cfg.SolanaConfig.RewardManagerLookupTable
	logger.Info("Fetching lookup table", zap.String("address", lookupTableAddress.String()))

	// Get current lookup table contents
	lookupTableAccount, err := rpcClient.GetAccountInfoWithOpts(ctx, lookupTableAddress, &rpc.GetAccountInfoOpts{
		Encoding: solana.EncodingBase64,
	})
	if err != nil {
		return fmt.Errorf("failed to get lookup table account: %w", err)
	}

	if lookupTableAccount == nil || lookupTableAccount.Value == nil {
		return fmt.Errorf("lookup table does not exist at %s", lookupTableAddress.String())
	}

	var lookupTable spl.AddressLookupTable
	err = lookupTable.UnmarshalWithDecoder(bin.NewBinDecoder(lookupTableAccount.Value.Data.GetBinary()))
	if err != nil {
		return fmt.Errorf("failed to decode lookup table: %w", err)
	}

	logger.Info("Found lookup table",
		zap.Int("existingAccounts", len(lookupTable.Addresses)),
	)

	// Build map of existing accounts for quick lookup
	existingAccounts := make(map[string]bool)
	for _, addr := range lookupTable.Addresses {
		existingAccounts[addr.String()] = true
	}

	logger.Info("Fetching registered validators...")
	validators, err := getValidators(ctx, openAudioSDK)
	if err != nil {
		return fmt.Errorf("failed to get registered validators: %w", err)
	}
	logger.Info("Found validators", zap.Int("count", len(validators)))

	// Derive sender addresses for validators
	authority := rewardManagerClient.GetAuthority()
	programID := cfg.SolanaConfig.RewardManagerProgramID

	var validatorSendersToAdd []solana.PublicKey
	for _, validator := range validators {
		addr := common.HexToAddress(validator.DelegateWallet)
		senderAddr, _, err := reward_manager.DeriveSenderAccount(programID, authority, addr)
		if err != nil {
			logger.Warn("Failed to derive sender account",
				zap.String("owner", validator.Owner),
				zap.String("ethAddress", validator.DelegateWallet),
				zap.String("endpoint", validator.Endpoint),
				zap.Error(err),
			)
			continue
		}

		if !existingAccounts[senderAddr.String()] {
			validatorSendersToAdd = append(validatorSendersToAdd, senderAddr)
			logger.Info("New validator sender to add",
				zap.String("owner", validator.Owner),
				zap.String("ethAddress", validator.DelegateWallet),
				zap.String("endpoint", validator.Endpoint),
				zap.String("sender", senderAddr.String()),
			)
		}
	}

	// Derive sender addresses for anti-abuse oracles
	logger.Debug("Fetching anti-abuse oracles...")
	oracles, err := getAntiAbuseOracles(cfg.AntiAbuseOracles)
	if err != nil {
		return fmt.Errorf("failed to get anti-abuse oracles: %w", err)
	}
	logger.Debug("Found anti-abuse oracles", zap.Int("count", len(oracles)))

	var oracleSendersToAdd []solana.PublicKey
	for _, oracleNode := range oracles {
		ownerAddr := common.HexToAddress(oracleNode.Owner)
		senderAddr, _, err := reward_manager.DeriveSenderAccount(programID, authority, ownerAddr)
		if err != nil {
			logger.Warn("Failed to derive oracle sender account",
				zap.String("owner", oracleNode.Owner),
				zap.Error(err),
			)
			continue
		}

		if !existingAccounts[senderAddr.String()] {
			oracleSendersToAdd = append(oracleSendersToAdd, senderAddr)
			logger.Info("New oracle sender to add",
				zap.String("owner", oracleNode.Owner),
				zap.String("sender", senderAddr.String()),
			)
		}
	}

	logger.Info("Summary",
		zap.Int("newValidatorSenders", len(validatorSendersToAdd)),
		zap.Int("newOracleSenders", len(oracleSendersToAdd)),
	)

	allAddressesToAdd := append(oracleSendersToAdd, validatorSendersToAdd...)

	if len(allAddressesToAdd) == 0 {
		logger.Info("No new senders to add. Lookup table is up to date.")
		return nil
	}

	if dryRun {
		logger.Info("Dry run mode - skipping lookup table update")
		return nil
	}

	// Extend lookup table in batches
	payer := cfg.SolanaConfig.FeePayers[0]
	transactionSender := spl.NewTransactionSender(cfg.SolanaConfig.FeePayers, cfg.SolanaConfig.RpcProviders)

	batchSize := 20
	for len(allAddressesToAdd) > 0 {
		batchEnd := batchSize
		if batchEnd > len(allAddressesToAdd) {
			batchEnd = len(allAddressesToAdd)
		}
		batch := allAddressesToAdd[:batchEnd]
		allAddressesToAdd = allAddressesToAdd[batchEnd:]

		logger.Info("Extending lookup table",
			zap.String("table", lookupTableAddress.String()),
			zap.Int("batchSize", len(batch)),
			zap.Int("remaining", len(allAddressesToAdd)),
		)

		extendInstruction := address_lookup_table.NewExtendLookupTableInstruction(
			payer.PublicKey(),
			lookupTableAddress,
			payer.PublicKey(),
			batch,
		).Build()

		tx := solana.NewTransactionBuilder()
		tx.SetFeePayer(payer.PublicKey())
		tx.AddInstruction(extendInstruction)

		err = transactionSender.AddPriorityFees(ctx, tx, spl.AddPriorityFeesParams{
			Percentile: 99,
			Multiplier: 1,
		})
		if err != nil {
			return fmt.Errorf("failed to add priority fees: %w", err)
		}

		logger.Info("Sending transaction...")
		signature, err := transactionSender.SendTransactionWithRetries(ctx, tx, rpc.CommitmentConfirmed, rpc.TransactionOpts{})
		if err != nil {
			return fmt.Errorf("failed to send transaction: %w", err)
		}

		logger.Info("Transaction confirmed", zap.String("signature", signature.String()))
	}

	logger.Info("Lookup table update complete")
	return nil
}
