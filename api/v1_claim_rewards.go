package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/solana/spl"
	"bridgerton.audius.co/solana/spl/programs/reward_manager"
	"bridgerton.audius.co/solana/spl/programs/secp256k1"
	"bridgerton.audius.co/trashid"
	"github.com/AudiusProject/audiusd/pkg/rewards"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type SenderAttestation struct {
	Message    []byte
	Signature  []byte
	EthAddress common.Address
}

const (
	validatorAttestationPath       = "/core/rewards/attestation"
	antiAbuseOracleAttestationPath = "/attestation"
)

var antiAbuseOracleMap map[string]string = make(map[string]string)

type GetAntiAbuseOracleAttestationParams struct {
	Claim                   rewards.RewardClaim
	Handle                  string
	AntiAbuseOracleEndpoint string
}
type AntiAbuseOracleAttestationRequestBody struct {
	ChallengeID string `json:"challengeId"`
	Specifier   string `json:"challengeSpecifier"`
	Amount      uint64 `json:"amount,string"`
}
type AntiAbuseOracleAttestationResponseBody struct {
	Result string `json:"result"`
}

// Gets a reward claim attestation from Anti Abuse Oracle
func getAntiAbuseOracleAttestation(args GetAntiAbuseOracleAttestationParams) (*SenderAttestation, error) {
	attestationBody := AntiAbuseOracleAttestationRequestBody{
		ChallengeID: args.Claim.RewardID,
		Specifier:   args.Claim.Specifier,
		Amount:      args.Claim.Amount,
	}

	reqBody, err := json.Marshal(attestationBody)
	if err != nil {
		return nil, err
	}

	base, err := url.Parse(args.AntiAbuseOracleEndpoint)
	if err != nil {
		return nil, err
	}

	pathname := path.Join(antiAbuseOracleAttestationPath, args.Handle)
	base.Path += pathname

	resp, err := http.Post(base.String(), "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get oracle attestation from %s. status %d: %s",
			args.AntiAbuseOracleEndpoint,
			resp.StatusCode,
			body,
		)
	}

	respBody := AntiAbuseOracleAttestationResponseBody{}
	err = json.Unmarshal(body, &respBody)
	if err != nil {
		return nil, err
	}
	address, exists := antiAbuseOracleMap[args.AntiAbuseOracleEndpoint]
	if !exists {
		return nil, fmt.Errorf("failed to find AAO address for %s", args.AntiAbuseOracleEndpoint)
	}
	message, err := args.Claim.Compile()
	if err != nil {
		return nil, err
	}

	// Pad the start if there's a missing leading zero
	signature := respBody.Result
	if len(signature)%2 == 1 {
		signature = "0" + signature
	}
	signatureBytes, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
	if err != nil {
		return nil, err
	}
	attestation := SenderAttestation{
		EthAddress: common.HexToAddress(address),
		Message:    message,
		Signature:  signatureBytes,
	}
	return &attestation, nil
}

type GetValidatorAttestationParams struct {
	Validator      string
	Claim          rewards.RewardClaim
	UserEthAddress string
	Signature      string
}

type ValidatorAttestationResponseBody struct {
	Attestation string `json:"attestation"`
	Owner       string `json:"owner"`
}

// Gets a reward claim attestation from a validator.
func getValidatorAttestation(args GetValidatorAttestationParams) (*SenderAttestation, error) {
	query := url.Values{}
	query.Add("reward_id", args.Claim.RewardID)
	query.Add("specifier", args.Claim.Specifier)
	query.Add("eth_recipient_address", args.Claim.RecipientEthAddress)
	query.Add("oracle_address", args.Claim.AntiAbuseOracleEthAddress)
	query.Add("amount", strconv.FormatUint(args.Claim.Amount, 10))
	query.Add("signature", args.Signature)

	base, err := url.Parse(args.Validator)
	if err != nil {
		return nil, err
	}

	base.Path += validatorAttestationPath
	base.RawQuery = query.Encode()

	resp, err := http.Get(base.String())
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("failed to get validator attestation from %s. status %d: %s",
			args.Validator,
			resp.StatusCode,
			body,
		)
	}
	respBody := ValidatorAttestationResponseBody{}
	err = json.Unmarshal(body, &respBody)
	if err != nil {
		return nil, err
	}

	// Pad the start if there's a missing leading zero
	signature := respBody.Attestation
	if len(signature)%2 == 1 {
		signature = "0" + signature
	}
	signatureBytes, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
	if err != nil {
		return nil, err
	}
	message, err := args.Claim.Compile()
	if err != nil {
		return nil, err
	}
	attestation := SenderAttestation{
		EthAddress: common.HexToAddress(respBody.Owner),
		Message:    message,
		Signature:  signatureBytes,
	}
	return &attestation, nil
}

// Gets reward claim attestations from AAO and Validators in parallel,
// handling retries and reselection
func fetchAttestations(
	ctx context.Context,
	rewardClaim RewardClaim,
	allValidators []config.Node,
	excludedOperators []string,
	antiAbuseOracle config.Node,
	signature string,
	hasAntiAbuseOracleAttestation bool,
	minVotes int,
) ([]SenderAttestation, error) {

	// Shuffle the validators
	shuffled := slices.Clone(allValidators)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	usedOwners := make(map[string]bool)
	for _, excluded := range excludedOperators {
		usedOwners[excluded] = true
	}
	badValidators := make(map[string]bool)

	offset := 0
	if !hasAntiAbuseOracleAttestation {
		offset = 1
	}

	attestations := make([]SenderAttestation, 0, minVotes+offset)

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Start AAO attestation in parallel if needed
	var aaoAttestation *SenderAttestation
	aaoGroup := &errgroup.Group{}

	if !hasAntiAbuseOracleAttestation {
		aaoGroup.Go(func() error {
			aaoClaim := rewardClaim.RewardClaim
			aaoClaim.AntiAbuseOracleEthAddress = ""
			getAntiAbuseAttestationParams := GetAntiAbuseOracleAttestationParams{
				Claim:                   aaoClaim,
				Handle:                  rewardClaim.Handle,
				AntiAbuseOracleEndpoint: antiAbuseOracle.Endpoint,
			}
			var err error
			aaoAttestation, err = getAntiAbuseOracleAttestation(getAntiAbuseAttestationParams)
			return err
		})
	}

	// Get validator attestations in parallel rounds
	successfulValidators := 0
	currentIndex := 0

	for successfulValidators < minVotes && currentIndex < len(shuffled) {
		// Select up to validatorsNeeded validators for this round
		var candidateNodes []config.Node
		var candidateIndices []int
		candidateOwners := make(map[string]bool) // Track owners in this round

		for i := currentIndex; i < len(shuffled) && len(candidateNodes) < minVotes; i++ {
			node := shuffled[i]
			// Skip if we've already used this owner globally or this specific validator is marked bad
			if usedOwners[node.OwnerWallet] || badValidators[node.Endpoint] {
				continue
			}
			// Skip if we've already picked this owner in this round
			if candidateOwners[node.OwnerWallet] {
				continue
			}
			candidateNodes = append(candidateNodes, node)
			candidateIndices = append(candidateIndices, i)
			candidateOwners[node.OwnerWallet] = true
		}

		if len(candidateNodes) == 0 {
			break // No more valid candidates
		}

		type validatorResult struct {
			index       int
			attestation *SenderAttestation
			err         error
		}
		results := make([]validatorResult, len(candidateNodes))
		validatorGroup := &errgroup.Group{}

		for i, node := range candidateNodes {
			i, node := i, node // capture loop variables
			validatorGroup.Go(func() error {
				getValidatorAttestationParams := GetValidatorAttestationParams{
					Validator:      node.Endpoint,
					Claim:          rewardClaim.RewardClaim,
					UserEthAddress: rewardClaim.RecipientEthAddress,
					Signature:      signature,
				}

				attestation, err := getValidatorAttestation(getValidatorAttestationParams)
				results[i] = validatorResult{
					index:       candidateIndices[i],
					attestation: attestation,
					err:         err,
				}
				return nil // Don't fail the group on individual validator errors
			})
		}

		validatorGroup.Wait()

		for _, result := range results {
			node := shuffled[result.index]
			if result.err != nil {
				badValidators[node.Endpoint] = true
				continue
			}

			// Success - add attestation and mark owner as used
			attestations = append(attestations, *result.attestation)
			usedOwners[node.OwnerWallet] = true
			successfulValidators++

			// Stop if we have enough validators
			if successfulValidators >= minVotes {
				break
			}
		}

		// Move to next batch of validators
		if len(candidateIndices) > 0 {
			currentIndex = candidateIndices[len(candidateIndices)-1] + 1
		} else {
			currentIndex = len(shuffled) // No more candidates found
		}
	}

	// Wait for AAO attestation to complete
	if err := aaoGroup.Wait(); err != nil {
		return nil, fmt.Errorf("failed to get anti-abuse oracle attestation: %w", err)
	}
	if aaoAttestation != nil {
		attestations = append([]SenderAttestation{*aaoAttestation}, attestations...)
	}

	if successfulValidators < minVotes {
		return nil, fmt.Errorf("could only get %d validator attestations, need %d", successfulValidators, validatorsNeeded)
	}

	return attestations, nil
}

// Builds a Solana transaction to claim a reward from the attestations and sends it with retries.
func sendRewardClaimTransactions(
	ctx context.Context,
	rewardManagerClient *reward_manager.RewardManagerClient,
	transactionSender *spl.TransactionSender,
	rewardClaim RewardClaim,
	attestations []SenderAttestation,
) ([]solana.Signature, error) {
	// Transaction to send attestations in a separate transaction
	partialTx := solana.NewTransactionBuilder()
	// Transaction to send attestations and evaluate in one transaction
	// If partialTx is sent, remainderTx contains only the rest of the instructions
	remainderTx := solana.NewTransactionBuilder()

	feePayer := transactionSender.GetFeePayer()
	partialTx.SetFeePayer(feePayer.PublicKey())
	remainderTx.SetFeePayer(feePayer.PublicKey())

	for i, attestation := range attestations {
		instructionIndex := uint8(i * 2)
		submitAttestationSecpInstruction := secp256k1.NewSecp256k1Instruction(
			attestation.EthAddress,
			attestation.Message,
			attestation.Signature,
			instructionIndex,
		).Build()
		submitAttestationInstruction, err := reward_manager.NewSubmitAttestationInstruction(
			rewardClaim.RewardID,
			rewardClaim.Specifier,
			attestation.EthAddress,
			rewardManagerClient.GetProgramStateAccount(),
			feePayer.PublicKey(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to build submitAttestation instruction: %w", err)
		}

		partialTx.AddInstruction(submitAttestationSecpInstruction)
		partialTx.AddInstruction(submitAttestationInstruction.Build())
		remainderTx.AddInstruction(submitAttestationSecpInstruction)
		remainderTx.AddInstruction(submitAttestationInstruction.Build())
	}

	lookupTable, err := rewardManagerClient.GetLookupTable(ctx)
	if err != nil {
		return nil, err
	}
	addressLookupTables := map[solana.PublicKey]solana.PublicKeySlice{
		rewardManagerClient.GetLookupTableAccount(): lookupTable.Addresses,
	}

	txSignatures := make([]solana.Signature, 0)

	// If no attestations need to be submitted, don't need to split into two txs
	if len(attestations) > 0 {
		partialTx.WithOpt(solana.TransactionAddressTables(addressLookupTables))
		err = transactionSender.AddPriorityFees(ctx, partialTx, spl.AddPriorityFeesParams{Percentile: 99, Multiplier: 1})
		if err != nil {
			return nil, err
		}
		err = transactionSender.AddComputeBudgetLimit(ctx, partialTx, spl.AddComputeBudgetLimitParams{Padding: 1000, Multiplier: 1.2})
		if err != nil {
			return nil, err
		}
		partialTxBuilt, err := partialTx.Build()
		if err != nil {
			return nil, err
		}

		partialTxBinary, err := partialTxBuilt.MarshalBinary()
		if err != nil {
			return nil, err
		}

		// Check to see if there's room for the evaluate instruction.
		// If not, send the attestations in a separate transaction.
		estimatedEvaluateInstructionSize := 205
		threshold := spl.MAX_TRANSACTION_SIZE - estimatedEvaluateInstructionSize
		if len(partialTxBinary) > threshold {
			sig, err := transactionSender.SendTransactionWithRetries(ctx, partialTx, rpc.CommitmentConfirmed, rpc.TransactionOpts{})
			if err != nil {
				return nil, err
			}
			txSignatures = append(txSignatures, *sig)
			remainderTx = solana.NewTransactionBuilder()
		}
	}

	state, err := rewardManagerClient.GetProgramState(ctx)
	if err != nil {
		return nil, err
	}
	evaluateAttestationInstruction, err := reward_manager.NewEvaluateAttestationInstruction(
		rewardClaim.RewardID,
		rewardClaim.Specifier,
		common.HexToAddress(rewardClaim.RecipientEthAddress),
		rewardClaim.Amount*1e8, // Convert to wAUDIO wei
		common.HexToAddress(rewardClaim.AntiAbuseOracleEthAddress),
		rewardManagerClient.GetProgramStateAccount(),
		state.TokenAccount,
		rewardClaim.UserBank,
		feePayer.PublicKey(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to build evaluateAttestation instruction: %w", err)
	}

	remainderTx.AddInstruction(evaluateAttestationInstruction.Build())

	remainderTx.WithOpt(solana.TransactionAddressTables(addressLookupTables))
	err = transactionSender.AddComputeBudgetLimit(ctx, remainderTx, spl.AddComputeBudgetLimitParams{Padding: 1000, Multiplier: 1.2})
	if err != nil {
		return nil, err
	}
	err = transactionSender.AddPriorityFees(ctx, remainderTx, spl.AddPriorityFeesParams{Percentile: 99, Multiplier: 1})
	if err != nil {
		return nil, err
	}

	sig, err := transactionSender.SendTransactionWithRetries(ctx, remainderTx, rpc.CommitmentConfirmed, rpc.TransactionOpts{})
	if err != nil {
		return nil, err
	}
	txSignatures = append(txSignatures, *sig)
	return txSignatures, nil
}

type RelayTransactionRequest struct {
	Transaction string `json:"transaction"`
}
type RelayTransactionResponse struct {
	Signature string `json:"signature"`
}

// Claims an individual reward.
func claimReward(
	ctx context.Context,
	rewardClaim RewardClaim,
	rewardManagerClient *reward_manager.RewardManagerClient,
	rewardAttester *rewards.RewardAttester,
	transactionSender *spl.TransactionSender,
	antiAbuseOracle config.Node,
	validators []config.Node,
) ([]solana.Signature, error) {

	rewardManagerStateData, err := rewardManagerClient.GetProgramState(ctx)
	if err != nil {
		return nil, err
	}

	attestationsData, err := rewardManagerClient.GetSubmittedAttestations(ctx, rewardClaim.RewardClaim)
	if err != nil {
		// If not found, then it's empty. Use default values for the purpose
		// of getting an empty list of messages
		if err.Error() != "not found" {
			return nil, err
		}
		attestationsData = &reward_manager.AttestationsAccountData{}
	}

	hasAntiAbuseOracleAttestation := false
	existingValidatorOwners := make([]string, 0)
	for _, attestation := range attestationsData.Messages {
		if attestation.Claim.AntiAbuseOracleEthAddress != "" {
			existingValidatorOwners = append(existingValidatorOwners, attestation.OperatorEthAddress)
		} else {
			hasAntiAbuseOracleAttestation = true
		}
	}

	// Attest from Bridge to get authority signature
	_, signature, err := rewardAttester.Attest(rewardClaim.RewardClaim)
	if err != nil {
		return nil, err
	}

	// Fetch AAO and validator attestations
	attestations, err := fetchAttestations(
		ctx,
		rewardClaim,
		validators,
		existingValidatorOwners,
		antiAbuseOracle,
		signature,
		hasAntiAbuseOracleAttestation,
		int(rewardManagerStateData.MinVotes),
	)
	if err != nil {
		return nil, err
	}

	// Build and send solana transactions
	signatures, err := sendRewardClaimTransactions(
		ctx,
		rewardManagerClient,
		transactionSender,
		rewardClaim,
		attestations,
	)
	if err != nil {
		return nil, err
	}

	return signatures, nil
}

type HealthCheckResponse struct {
	AntiAbuseWalletPubkey string
}

// Selects a healthy Anti Abuse Oracle and gets its address.
// TODO: Implement AAO in bridge and use config
func getAntiAbuseOracle(antiAbuseOracleEndpoints []string) (node *config.Node, err error) {
	oracleEndpoint := antiAbuseOracleEndpoints[rand.IntN(len(antiAbuseOracleEndpoints))]

	if value, exists := antiAbuseOracleMap[oracleEndpoint]; exists {
		return &config.Node{
			DelegateOwnerWallet: value,
			Endpoint:            oracleEndpoint,
		}, nil
	}

	resp, err := http.Get(oracleEndpoint + "/health_check")
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to get oracle from "+oracleEndpoint+". Error "+resp.Status+": "+string(body))
	}
	health := &HealthCheckResponse{}
	err = json.Unmarshal(body, health)
	if err != nil {
		return nil, err
	}

	antiAbuseOracleMap[oracleEndpoint] = health.AntiAbuseWalletPubkey
	return &config.Node{
		DelegateOwnerWallet: health.AntiAbuseWalletPubkey,
		Endpoint:            oracleEndpoint,
	}, nil
}

func getReward(rewardId string, rewardsList []rewards.Reward) (rewards.Reward, error) {
	for _, r := range rewardsList {
		if r.RewardId == rewardId {
			return r, nil
		}
	}
	return rewards.Reward{}, fmt.Errorf("challenge ID %s does not have a configured reward", rewardId)
}

type RewardClaim struct {
	rewards.RewardClaim
	Handle   string
	UserBank solana.PublicKey
}

type ClaimResult struct {
	ChallengeID string             `json:"challengeId"`
	Specifier   string             `json:"specifier"`
	Amount      uint64             `json:"amount"`
	Signatures  []solana.Signature `json:"signatures"`
	Error       string             `json:"error,omitempty"`
}

type ClaimRewardsBody struct {
	ChallengeID string `json:"challengeId"`
	Specifier   string `json:"specifier"`
	UserID      string `json:"userId"`
}

// Claims all the filtered undisbursed rewards for a user.
func (app *ApiServer) v1ClaimRewards(c *fiber.Ctx) error {

	body := ClaimRewardsBody{}
	err := c.BodyParser(&body)
	if err != nil {
		return err
	}
	hashId := body.UserID
	challengeId := body.ChallengeID
	specifier := body.Specifier

	if hashId == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing user ID")
	}

	userId, err := trashid.DecodeHashId(hashId)
	if err != nil {
		return err
	}

	undisbursedRows, err := app.queries.GetUndisbursedChallenges(
		c.Context(),
		dbv1.GetUndisbursedChallengesParams{
			UserID:      int32(userId),
			ChallengeID: challengeId,
			Specifier:   specifier,
		})

	if err != nil {
		return err
	}

	if len(undisbursedRows) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "No rewards to claim")
	}

	antiAbuseOracle, err := getAntiAbuseOracle(app.antiAbuseOracles)
	if err != nil {
		return err
	}

	bankAccount, err := app.claimableTokensClient.GetOrCreateUserBank(
		c.Context(),
		common.HexToAddress(undisbursedRows[0].Wallet.String),
		app.solanaConfig.MintAudio,
	)
	if err != nil {
		return err
	}

	results := make([]ClaimResult, len(undisbursedRows))
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	g := &sync.WaitGroup{}
	g.Add(len(undisbursedRows))
	for i, row := range undisbursedRows {
		go func() {
			results[i] = ClaimResult{
				ChallengeID: row.ChallengeID,
				Specifier:   row.Specifier,
			}

			reward, err := getReward(row.ChallengeID, app.rewardAttester.Rewards)
			if err != nil {
				results[i].Error = err.Error()
				g.Done()
				return
			}

			results[i].Amount = reward.Amount

			rewardClaim := RewardClaim{
				RewardClaim: rewards.RewardClaim{
					RewardID:                  row.ChallengeID,
					Amount:                    reward.Amount,
					Specifier:                 row.Specifier,
					RecipientEthAddress:       row.Wallet.String,
					AntiAbuseOracleEthAddress: antiAbuseOracle.DelegateOwnerWallet,
				},
				Handle:   row.Handle.String,
				UserBank: *bankAccount,
			}

			sigs, err := claimReward(
				ctx,
				rewardClaim,
				app.rewardManagerClient,
				app.rewardAttester,
				app.transactionSender,
				*antiAbuseOracle,
				app.validators,
			)

			if err != nil {
				var instrErr *spl.InstructionError
				if errors.As(err, &instrErr) {
					app.logger.Error("failed to claim challenge reward. transaction failed to send.",
						zap.String("handle", row.Handle.String),
						zap.String("rewardId", row.ChallengeID),
						zap.String("specifier", row.Specifier),
						zap.String("transaction", instrErr.EncodedTransaction),
						zap.String("customError", reward_manager.RewardManagerError(instrErr.Code).String()),
						zap.Error(err),
					)
				} else {
					app.logger.Error("failed to claim challenge reward.",
						zap.String("handle", row.Handle.String),
						zap.String("rewardId", row.ChallengeID),
						zap.String("specifier", row.Specifier),
						zap.Error(err),
					)
				}
				results[i].Error = err.Error()
			}

			results[i].Signatures = sigs
			g.Done()
		}()
	}
	g.Wait()

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"data": results,
	})
}
