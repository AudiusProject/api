package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"

	"api.audius.co/api"
	"api.audius.co/config"
	"api.audius.co/logging"
	"api.audius.co/solana/spl"
	"api.audius.co/solana/spl/programs/reward_manager"
	"api.audius.co/solana/spl/programs/secp256k1"
	"connectrpc.com/connect"
	"github.com/AlecAivazis/survey/v2"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	ethv1 "github.com/OpenAudio/go-openaudio/pkg/api/eth/v1"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
	"github.com/ethereum/go-ethereum/common"
	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// ValidatorSenderInfo contains information about a validator and its sender account
type ValidatorSenderInfo struct {
	Validator *config.Node
	Sender    *SenderWithPubkey
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Register all senders for validators",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get flags
		rpcOverride, _ := cmd.Flags().GetString("rpc")
		openAudioURLOverride, _ := cmd.Flags().GetString("openAudioURL")
		dryRun, _ := cmd.Flags().GetBool("dry-run")
		keypairPath, _ := cmd.Flags().GetString("keypair")

		// Initialize logger
		cfg := config.Cfg
		logger := logging.NewZapLogger(cfg)
		logger.Info("Starting register_senders",
			zap.Bool("dryRun", dryRun),
		)

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
		return RunRegisterSenders(cmd.Context(), cfg, dryRun, logger)
	},
}

func init() {
	rootCmd.AddCommand(syncCmd)
}

func RunRegisterSenders(ctx context.Context, cfg config.Config, dryRun bool, logger *zap.Logger) error {
	logger.Debug("Initializing clients...")
	if len(cfg.OpenAudioURLs) == 0 {
		return fmt.Errorf("no OpenAudio URLs configured")
	}
	openAudioSDK := sdk.NewOpenAudioSDK(cfg.OpenAudioURLs[0])

	if len(cfg.SolanaConfig.RpcProviders) == 0 {
		return fmt.Errorf("no Solana RPC providers configured")
	}
	rpcClient := rpc.New(cfg.SolanaConfig.RpcProviders[0])

	// Ensure we use the right program ID for the environment
	reward_manager.SetProgramID(cfg.SolanaConfig.RewardManagerProgramID)

	if len(cfg.SolanaConfig.FeePayers) == 0 {
		return fmt.Errorf("no fee payer wallets configured")
	}
	payer := cfg.SolanaConfig.FeePayers[0]
	transactionSender := spl.NewTransactionSender(cfg.SolanaConfig.FeePayers, cfg.SolanaConfig.RpcProviders)

	logger.Debug("Fetching oracles...")
	oracles, err := getAntiAbuseOracles(cfg.AntiAbuseOracles)
	if err != nil {
		logger.Warn("Failed to get anti-abuse oracles", zap.Error(err))
		if !dryRun {
			var continueWithout bool
			prompt := &survey.Confirm{
				Message: fmt.Sprintf("Failed to get some anti-abuse oracles: %v. Continue without them?", err),
				Default: false,
			}
			if err := survey.AskOne(prompt, &continueWithout); err != nil || !continueWithout {
				return fmt.Errorf("failed to get anti-abuse oracles: %w", err)
			}
		}
		logger.Info("Continuing without missing anti-abuse oracles. This may result in attempting to register/deregister oracle senders.")
	}
	logger.Debug("Found oracles", zap.Int("count", len(oracles)))

	// Get list of validators and senders and diff them
	logger.Debug("Fetching validators...")
	validators, err := getValidators(ctx, openAudioSDK)
	if err != nil {
		return fmt.Errorf("failed to get validators: %w", err)
	}
	logger.Debug("Found validators", zap.Int("count", len(validators)))

	logger.Debug("Fetching senders...")
	senders, err := getSenders(ctx, rpcClient, reward_manager.ProgramID, cfg.SolanaConfig.RewardManagerState)
	if err != nil {
		return fmt.Errorf("failed to get senders: %w", err)
	}
	logger.Debug("Found senders", zap.Int("count", len(senders)))

	// Map validators and senders by delegate wallet / eth address
	mapped := make(map[string]ValidatorSenderInfo)
	for _, v := range validators {
		mapped[v.DelegateWallet] = ValidatorSenderInfo{
			Validator: &v,
		}
	}
	for _, s := range senders {
		if existing, ok := mapped[s.SenderEthAddress.String()]; ok {
			mapped[s.SenderEthAddress.String()] = ValidatorSenderInfo{
				Validator: existing.Validator,
				Sender:    &s,
			}
		} else {
			mapped[s.SenderEthAddress.String()] = ValidatorSenderInfo{
				Sender: &s,
			}
		}
	}

	toDeregister := make([]ValidatorSenderInfo, 0)
	toRegister := make([]ValidatorSenderInfo, 0)
	validSenders := make([]ValidatorSenderInfo, 0)
	for _, info := range mapped {
		if info.Validator == nil {
			if slices.ContainsFunc(oracles, func(n config.Node) bool {
				return n.DelegateWallet == info.Sender.SenderEthAddress.String()
			}) {
				logger.Debug("Skipping deregistration of oracle sender",
					zap.String("sender", info.Sender.Pubkey.String()),
					zap.String("owner", info.Sender.OwnerEthAddress.String()),
					zap.String("ethAddress", info.Sender.SenderEthAddress.String()),
				)
				validSenders = append(validSenders, info)
				continue
			}
			toDeregister = append(toDeregister, info)
		} else if info.Sender == nil {
			toRegister = append(toRegister, info)
		} else {
			validSenders = append(validSenders, info)
		}
	}

	registered := 0
	for _, info := range toRegister {
		logger := logger.With(
			zap.String("owner", info.Validator.Owner),
			zap.String("ethAddress", info.Validator.DelegateWallet),
			zap.String("endpoint", info.Validator.Endpoint),
		)

		if dryRun {
			logger.Info("Would register sender for validator")
			continue
		}

		logger.Debug("Registering sender for validator")

		attestations, err := getSenderAttestations(ctx, cfg.ArtistCoinRewardsStaticSenders, info.Validator.DelegateWallet, cfg.SolanaConfig.RewardManagerState, logger)
		if err != nil {
			logger.Error("Failed to get sender attestations", zap.Error(err))
			continue
		}

		tx, err := buildCreateSenderPublicTransaction(
			cfg.SolanaConfig.RewardManagerState, payer.PublicKey(), info.Validator, attestations,
		)
		if err != nil {
			logger.Error("Failed to build CreateSenderPublic transaction", zap.Error(err))
			continue
		}

		err = transactionSender.AddPriorityFees(ctx, tx, spl.AddPriorityFeesParams{})
		if err != nil {
			logger.Error("Failed to add priority fees to transaction", zap.Error(err))
			continue
		}
		tx.SetFeePayer(payer.PublicKey())

		sig, err := transactionSender.SendTransactionWithRetries(
			ctx, tx, rpc.CommitmentConfirmed, rpc.TransactionOpts{})
		if err != nil {
			logger.Error("Failed to send CreateSenderPublic transaction", zap.Error(err))
			continue
		}
		logger.Info("Successfully registered sender for validator", zap.String("signature", sig.String()))
		registered++
	}

	deregistered := 0
	for _, info := range toDeregister {
		logger := logger.With(
			zap.String("sender", info.Sender.Pubkey.String()),
			zap.String("owner", info.Sender.OwnerEthAddress.String()),
			zap.String("ethAddress", info.Sender.SenderEthAddress.String()),
		)

		if dryRun {
			logger.Info("Would deregister sender for validator")
			continue
		}

		logger.Warn("Not deregistering sender for validator - deregistration not yet implemented")
	}

	logger.Info("Summary",
		zap.Int("totalValidators", len(mapped)),
		zap.Int("neededRegistration", len(toRegister)),
		zap.Int("registered", registered),
		zap.Int("neededDeregistration", len(toDeregister)),
		zap.Int("deregistered", deregistered),
		zap.Int("noChange", len(validSenders)),
	)

	return nil
}

func getValidators(ctx context.Context, sdk *sdk.OpenAudioSDK) ([]config.Node, error) {
	resp, err := sdk.Eth.GetRegisteredEndpoints(ctx, connect.NewRequest(&ethv1.GetRegisteredEndpointsRequest{}))
	if err != nil {
		return nil, fmt.Errorf("failed to get registered endpoints: %w", err)
	}
	if resp == nil || resp.Msg == nil {
		return nil, fmt.Errorf("GetRegisteredEndpoints returned nil response")
	}

	var nodes []config.Node
	for _, node := range resp.Msg.Endpoints {
		nodes = append(nodes, config.Node{
			Id:             fmt.Sprintf("%d", node.Id),
			Endpoint:       node.Endpoint,
			DelegateWallet: node.DelegateWallet,
			Owner:          node.Owner,
			ServiceType:    node.ServiceType,
		})
	}

	return nodes, nil
}

type SenderWithPubkey struct {
	Pubkey solana.PublicKey
	reward_manager.SenderAccountData
}

func getSenders(
	ctx context.Context,
	rpcClient *rpc.Client,
	programId solana.PublicKey,
	rewardManagerState solana.PublicKey,
) ([]SenderWithPubkey, error) {
	res, err := rpcClient.GetProgramAccountsWithOpts(ctx, programId, &rpc.GetProgramAccountsOpts{
		Encoding: solana.EncodingBase64,
		Filters: []rpc.RPCFilter{
			{
				DataSize: 73, // Size of sender account
			},
			{
				Memcmp: &rpc.RPCFilterMemcmp{
					Offset: 1, // Offset of is_validator field
					Bytes:  rewardManagerState[:],
				},
			},
		},
	})
	if err != nil {
		return nil, err
	}

	out := make([]SenderWithPubkey, 0, len(res))
	for _, acc := range res {
		var data reward_manager.SenderAccountData
		err := bin.NewBinDecoder(acc.Account.Data.GetBinary()).Decode(&data)
		if err != nil {
			return nil, err
		}
		out = append(out, SenderWithPubkey{
			Pubkey:            acc.Pubkey,
			SenderAccountData: data,
		})
	}
	return out, nil
}

func getAntiAbuseOracles(antiAbuseOracleEndpoints []string) ([]config.Node, error) {
	oracles := make([]config.Node, len(antiAbuseOracleEndpoints))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errs []error

	for i, endpoint := range antiAbuseOracleEndpoints {
		wg.Go(func() {
			resp, err := http.Get(endpoint + "/health_check")
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to fetch from %s: %w", endpoint, err))
				mu.Unlock()
				return
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to read response from %s: %w", endpoint, err))
				mu.Unlock()
				return
			}

			if resp.StatusCode != 200 {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to get oracle from %s: %s: %s", endpoint, resp.Status, string(body)))
				mu.Unlock()
				return
			}

			health := &api.HealthCheckResponse{}
			err = json.Unmarshal(body, health)
			if err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("failed to unmarshal response from %s: %w", endpoint, err))
				mu.Unlock()
				return
			}

			oracles[i] = config.Node{
				Endpoint:       endpoint,
				DelegateWallet: health.AntiAbuseWalletPubkey,
				Owner:          health.AntiAbuseWalletPubkey,
				ServiceType:    "anti_abuse_oracle",
			}
		})
	}

	wg.Wait()

	if len(errs) > 0 {
		return oracles, fmt.Errorf("encountered %d errors: %v", len(errs), errs)
	}

	return oracles, nil
}

func getSenderAttestation(ctx context.Context, node config.Node, senderEthAddress string, rewardManagerState solana.PublicKey) (*corev1.GetRewardSenderAttestationResponse, error) {
	openAudioSdk := sdk.NewOpenAudioSDK(node.Endpoint)
	res, err := openAudioSdk.Core.GetRewardSenderAttestation(ctx, connect.NewRequest(&corev1.GetRewardSenderAttestationRequest{
		Address:              senderEthAddress,
		RewardsManagerPubkey: rewardManagerState.String(),
	}))
	if err != nil {
		return nil, fmt.Errorf("failed to get reward sender attestation from OpenAudio: %w", err)
	}
	if res == nil || res.Msg == nil {
		return nil, fmt.Errorf("GetRewardSenderAttestation returned nil response")
	}
	return res.Msg, nil
}

func getSenderAttestations(ctx context.Context, nodes []config.Node, senderEthAddress string, rewardManagerState solana.PublicKey, logger *zap.Logger) (map[string]*corev1.GetRewardSenderAttestationResponse, error) {
	attestations := make(map[string]*corev1.GetRewardSenderAttestationResponse)
	mapped := make(map[string][]config.Node)
	for _, node := range nodes {
		mapped[node.Owner] = append(mapped[node.Owner], node)
	}
	batchSize := 3
	maxAttempts := 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	for j := 0; j < maxAttempts && len(attestations) < 3 && len(mapped) > 0; j++ {
		i := 0
		for _, nodeList := range mapped {
			node := nodeList[0]
			nl := nodeList
			i++
			wg.Go(func() {
				attestation, err := getSenderAttestation(ctx, node, senderEthAddress, rewardManagerState)
				if err != nil {
					logger.Warn("Failed to get attestation", zap.String("from", node.Endpoint), zap.Error(err))
					return
				}

				mu.Lock()

				attestations[node.DelegateWallet] = attestation

				if len(nl) > 1 {
					mapped[node.Owner] = nl[1:]
				} else {
					delete(mapped, node.Owner)
				}

				mu.Unlock()
			})

			if i >= batchSize {
				wg.Wait()
				i = 0
			}

			if len(attestations) >= 3 {
				break
			}
		}
		wg.Wait()
	}
	if len(attestations) < 3 {
		return nil, fmt.Errorf("failed to get enough attestations for sender %s", senderEthAddress)
	}
	return attestations, nil
}

func buildCreateSenderPublicTransaction(
	rewardManagerState solana.PublicKey,
	payer solana.PublicKey,
	validator *config.Node,
	attestations map[string]*corev1.GetRewardSenderAttestationResponse,
) (*solana.TransactionBuilder, error) {
	tx := solana.NewTransactionBuilder()

	ethAddress := common.HexToAddress(validator.DelegateWallet)
	operatorEthAddress := common.HexToAddress(validator.Owner)

	// Build Secp256k1 instructions
	var b bytes.Buffer
	b.WriteString("add")
	b.Write(rewardManagerState[:])
	b.Write(ethAddress[:])
	message := b.Bytes()
	i := uint8(0)
	for _, attestation := range attestations {
		sig := common.Hex2Bytes(attestation.Attestation)
		inst := secp256k1.NewSecp256k1Instruction(common.HexToAddress(attestation.Owner), message, sig, i)
		tx.AddInstruction(inst.Build())
		i++
	}

	attesterEthAddresses := make([]common.Address, 0, len(attestations))
	for ethAddr := range attestations {
		attesterEthAddresses = append(attesterEthAddresses, common.HexToAddress(ethAddr))
	}
	createSenderPublicInst, err := reward_manager.NewCreateSenderPublicInstruction(ethAddress, operatorEthAddress, rewardManagerState, payer, attesterEthAddresses...)
	if err != nil {
		return nil, fmt.Errorf("failed to build CreateSenderPublic instruction: %w", err)
	}
	tx.AddInstruction(createSenderPublicInst.Build())

	return tx, nil
}
