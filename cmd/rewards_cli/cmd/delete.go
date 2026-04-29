package cmd

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"sync"

	"api.audius.co/config"
	"api.audius.co/logging"
	"api.audius.co/solana/spl"
	"api.audius.co/solana/spl/programs/reward_manager"
	"api.audius.co/solana/spl/programs/secp256k1"
	"connectrpc.com/connect"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/OpenAudio/go-openaudio/pkg/rewards"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var deleteCmd = &cobra.Command{
	Use:   "delete [address]",
	Short: "Remove a registered sender from the Reward Manager program",
	Args:  cobra.ExactArgs(1),
	RunE:  runDelete,
}

func init() {
	deleteCmd.Flags().StringSlice("nodes", nil, "Comma-separated OpenAudio node URLs to fetch delete attestations from (overrides config). Need ≥3 with distinct attester owners.")
	deleteCmd.Flags().String("refunder", "", "Solana pubkey to receive the reclaimed sender-account rent (defaults to fee payer)")
	rootCmd.AddCommand(deleteCmd)
}

func runDelete(cmd *cobra.Command, args []string) error {
	rpcOverride, _ := cmd.Flags().GetString("rpc")
	openAudioURLOverride, _ := cmd.Flags().GetString("openAudioURL")
	keypairPath, _ := cmd.Flags().GetString("keypair")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	nodeOverrides, _ := cmd.Flags().GetStringSlice("nodes")
	refunderFlag, _ := cmd.Flags().GetString("refunder")

	cfg := config.Cfg
	logger := logging.NewZapLogger(cfg)

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

	address := strings.TrimSpace(args[0])
	if !common.IsHexAddress(address) {
		return fmt.Errorf("%q is not a valid Ethereum address", address)
	}
	senderEthAddress := common.HexToAddress(address)

	ctx := cmd.Context()
	payer := cfg.SolanaConfig.FeePayers[0]

	refunder := payer.PublicKey()
	if refunderFlag != "" {
		refunder = solana.MustPublicKeyFromBase58(refunderFlag)
	}

	attesterNodes := cfg.ArtistCoinRewardsStaticSenders
	if len(nodeOverrides) > 0 {
		attesterNodes = make([]config.Node, 0, len(nodeOverrides))
		for _, ep := range nodeOverrides {
			ep = strings.TrimSpace(ep)
			if ep == "" {
				continue
			}
			attesterNodes = append(attesterNodes, config.Node{Endpoint: ep})
		}
	}
	if len(attesterNodes) < 3 {
		return fmt.Errorf("need at least 3 attester nodes, got %d", len(attesterNodes))
	}

	transactionSender := spl.NewTransactionSender(cfg.SolanaConfig.FeePayers, cfg.SolanaConfig.RpcProviders)

	logger.Info("Gathering delete attestations",
		zap.String("sender", senderEthAddress.Hex()),
		zap.String("rewardManagerState", cfg.SolanaConfig.RewardManagerState.String()),
		zap.Int("nodes", len(attesterNodes)))

	attestations, err := getDeleteSenderAttestations(ctx, attesterNodes, senderEthAddress.Hex(), cfg.SolanaConfig.RewardManagerState, logger)
	if err != nil {
		return fmt.Errorf("failed to get delete attestations: %w", err)
	}
	if len(attestations) < 3 {
		return fmt.Errorf("only collected %d distinct-owner delete attestations, need ≥3", len(attestations))
	}
	logger.Info("Got delete attestations", zap.Int("count", len(attestations)))

	for owner, a := range attestations {
		senderPDA, _, err := reward_manager.DeriveSenderAccount(reward_manager.ProgramID, cfg.SolanaConfig.RewardManagerState, common.HexToAddress(a.Owner))
		if err != nil {
			return fmt.Errorf("failed to derive sender account for attester %s: %w", a.Owner, err)
		}
		logger.Debug("Attestation",
			zap.String("attesterOwner", owner),
			zap.String("attesterEthAddress", a.Owner),
			zap.String("attesterSenderPDA", senderPDA.String()))
	}

	tx, err := buildDeleteSenderPublicTransaction(
		cfg.SolanaConfig.RewardManagerState,
		senderEthAddress,
		refunder,
		attestations,
	)
	if err != nil {
		return fmt.Errorf("failed to build delete sender transaction: %w", err)
	}

	if err := transactionSender.AddPriorityFees(ctx, tx, spl.AddPriorityFeesParams{}); err != nil {
		return fmt.Errorf("failed to add priority fees: %w", err)
	}

	tx.SetFeePayer(payer.PublicKey())
	txBuilt, err := tx.Build()
	if err != nil {
		return fmt.Errorf("failed to build transaction: %w", err)
	}
	fmt.Println(txBuilt.String())

	if dryRun {
		logger.Info("Dry run — not sending transaction")
		return nil
	}

	logger.Info("Sending delete sender transaction...")
	sig, err := transactionSender.SendTransactionWithRetries(
		ctx, tx, rpc.CommitmentConfirmed, rpc.TransactionOpts{})
	if err != nil {
		return fmt.Errorf("failed to send delete sender transaction: %w", err)
	}
	logger.Info("Successfully deleted sender",
		zap.String("sender", senderEthAddress.Hex()),
		zap.String("signature", sig.String()))

	return nil
}

func getDeleteSenderAttestation(ctx context.Context, node config.Node, senderEthAddress string, rewardManagerState solana.PublicKey) (*corev1.GetDeleteRewardSenderAttestationResponse, error) {
	openAudioSdk := sdk.NewOpenAudioSDK(node.Endpoint)
	res, err := openAudioSdk.Core.GetDeleteRewardSenderAttestation(ctx, connect.NewRequest(&corev1.GetDeleteRewardSenderAttestationRequest{
		Address:              senderEthAddress,
		RewardsManagerPubkey: rewardManagerState.String(),
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to get delete sender attestation from OpenAudio: %w", err)
	}
	if res == nil || res.Msg == nil {
		return nil, fmt.Errorf("GetDeleteRewardSenderAttestation returned nil response")
	}
	return res.Msg, nil
}

// getDeleteSenderAttestations fans out to every passed node in parallel,
// then dedupes responses by attester eth address. Returns attestations keyed
// by lowercased attester eth address. Caller is responsible for picking
// nodes from distinct operators; the dedupe guards against accidental dupes.
func getDeleteSenderAttestations(ctx context.Context, nodes []config.Node, senderEthAddress string, rewardManagerState solana.PublicKey, logger *zap.Logger) (map[string]*corev1.GetDeleteRewardSenderAttestationResponse, error) {
	type result struct {
		node        config.Node
		attestation *corev1.GetDeleteRewardSenderAttestationResponse
		err         error
	}
	results := make([]result, len(nodes))
	var wg sync.WaitGroup
	for i, node := range nodes {
		idx, n := i, node
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, err := getDeleteSenderAttestation(ctx, n, senderEthAddress, rewardManagerState)
			results[idx] = result{node: n, attestation: a, err: err}
		}()
	}
	wg.Wait()

	attestations := make(map[string]*corev1.GetDeleteRewardSenderAttestationResponse)
	for _, r := range results {
		if r.err != nil {
			logger.Warn("Failed to get delete attestation",
				zap.String("from", r.node.Endpoint),
				zap.Error(r.err))
			continue
		}
		key := strings.ToLower(r.attestation.Owner)
		if _, dup := attestations[key]; dup {
			logger.Warn("Skipping duplicate-attester attestation (two nodes signed with the same key)",
				zap.String("from", r.node.Endpoint),
				zap.String("attester", r.attestation.Owner))
			continue
		}
		attestations[key] = r.attestation
	}
	return attestations, nil
}

func buildDeleteSenderPublicTransaction(
	rewardManagerState solana.PublicKey,
	senderEthAddress common.Address,
	refunder solana.PublicKey,
	attestations map[string]*corev1.GetDeleteRewardSenderAttestationResponse,
) (*solana.TransactionBuilder, error) {
	tx := solana.NewTransactionBuilder()

	// Each secp256k1 instruction signs the exact 55-byte payload the
	// rewards-manager program reconstructs and validates on-chain.
	var b bytes.Buffer
	b.WriteString(rewards.DeleteSenderMessagePrefix)
	b.Write(rewardManagerState[:])
	b.Write(senderEthAddress[:])
	message := b.Bytes()

	attesterEthAddresses := make([]common.Address, 0, len(attestations))
	i := uint8(0)
	for _, attestation := range attestations {
		sig := common.Hex2Bytes(attestation.Attestation)
		attesterEth := common.HexToAddress(attestation.Owner)
		secpInst := secp256k1.NewSecp256k1Instruction(attesterEth, message, sig, i)
		tx.AddInstruction(secpInst.Build())
		attesterEthAddresses = append(attesterEthAddresses, attesterEth)
		i++
	}

	deleteInst, err := reward_manager.NewDeleteSenderPublicInstruction(
		senderEthAddress,
		rewardManagerState,
		refunder,
		attesterEthAddresses...,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build DeleteSenderPublic instruction: %w", err)
	}
	tx.AddInstruction(deleteInst.Build())

	return tx, nil
}
