package cmd

import (
	"fmt"

	"api.audius.co/config"
	"api.audius.co/logging"
	"api.audius.co/solana/spl"
	"api.audius.co/solana/spl/programs/reward_manager"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var registerCmd = &cobra.Command{
	Use:   "register [address] [operator]",
	Short: "Register a single Ethereum address as a sender in the rewards program",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get flags
		rpcOverride, _ := cmd.Flags().GetString("rpc")
		openAudioURLOverride, _ := cmd.Flags().GetString("openAudioURL")
		keypairPath, _ := cmd.Flags().GetString("keypair")
		dryRun, _ := cmd.Flags().GetBool("dry-run")

		if dryRun {
			return fmt.Errorf("dry-run mode not supported for register command")
		}

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

		address := args[0]
		operator := args[1]

		ctx := cmd.Context()
		payer := cfg.SolanaConfig.FeePayers[0]
		validators := cfg.ArtistCoinRewardsStaticSenders
		transactionSender := spl.NewTransactionSender(cfg.SolanaConfig.FeePayers, cfg.SolanaConfig.RpcProviders)

		logger.Debug("Getting attestations...")
		attestations, err := getSenderAttestations(ctx, validators, address, cfg.SolanaConfig.RewardManagerState, logger)
		if err != nil {
			return fmt.Errorf("failed to get sender attestations: %w", err)
		}
		logger.Debug("Got attestations", zap.Int("count", len(attestations)))

		for _, a := range attestations {
			sender, _, err := reward_manager.DeriveSenderAccount(reward_manager.ProgramID, cfg.SolanaConfig.RewardManagerState, common.HexToAddress(a.Owner))
			if err != nil {
				return fmt.Errorf("failed to derive sender account: %w", err)
			}
			logger.Debug("Attestation",
				zap.String("attester", a.Owner),
				zap.String("senderAccount", sender.String()))
		}

		logger.Debug("Building create sender transaction...")
		tx, err := buildCreateSenderPublicTransaction(
			cfg.SolanaConfig.RewardManagerState,
			payer.PublicKey(),
			&config.Node{DelegateWallet: address, Owner: operator},
			attestations,
		)
		if err != nil {
			return fmt.Errorf("failed to build create sender transaction: %w", err)
		}

		logger.Debug("Adding priority fees to transaction...")
		err = transactionSender.AddPriorityFees(ctx, tx, spl.AddPriorityFeesParams{})
		if err != nil {
			return fmt.Errorf("failed to add priority fees: %w", err)
		}
		logger.Debug("Priority fees added")

		tx.SetFeePayer(payer.PublicKey())
		txBuilt, err := tx.Build()
		if err != nil {
			return fmt.Errorf("failed to build transaction: %w", err)
		}
		fmt.Println(txBuilt.String())

		logger.Info("Sending create sender transaction...")
		sig, err := transactionSender.SendTransactionWithRetries(
			ctx, tx, rpc.CommitmentConfirmed, rpc.TransactionOpts{})
		if err != nil {
			return fmt.Errorf("failed to send create sender transaction: %w", err)
		}
		logger.Info("Successfully registered sender", zap.String("signature", sig.String()))

		return nil
	},
}

func init() {
	rootCmd.AddCommand(registerCmd)
}
