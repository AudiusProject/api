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
	"bridgerton.audius.co/api/spl"
	"bridgerton.audius.co/api/spl/programs/reward_manager"
	"bridgerton.audius.co/api/spl/programs/secp256k1"
	"bridgerton.audius.co/config"
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

// Selects three uniquely owned, healthy validators at random
// TODO: add health checks?
func getValidators(validators []config.Node, count int, excludedOperators []string) ([]string, error) {
	shuffled := slices.Clone(validators)
	rand.Shuffle(len(validators), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	selected := make([]string, 0)
	for i := 0; i < len(shuffled) && len(selected) < count; i++ {
		node := shuffled[i]
		if !slices.Contains(excludedOperators, node.OwnerWallet) {
			selected = append(selected, node.Endpoint)
			excludedOperators = append(excludedOperators, node.OwnerWallet)
		}
	}
	return selected, nil
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

// Gets reward claim attestations from AAO and Validators in parallel.
func fetchAttestations(
	ctx context.Context,
	rewardClaim RewardClaim,
	validators []string,
	antiAbuseOracle config.Node,
	signature string,
	hasAntiAbuseOracleAttestation bool,
) ([]SenderAttestation, error) {

	offset := 0
	if !hasAntiAbuseOracleAttestation {
		offset = 1
	}
	attestations := make([]SenderAttestation, len(validators)+offset)

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	innerGroup, _ := errgroup.WithContext(ctx)

	if !hasAntiAbuseOracleAttestation {
		innerGroup.Go(func() error {
			aaoClaim := rewardClaim.RewardClaim
			aaoClaim.AntiAbuseOracleEthAddress = ""
			getAntiAbuseAttestationParams := GetAntiAbuseOracleAttestationParams{
				Claim:                   aaoClaim,
				Handle:                  rewardClaim.Handle,
				AntiAbuseOracleEndpoint: antiAbuseOracle.Endpoint,
			}
			aaoAttestation, err := getAntiAbuseOracleAttestation(getAntiAbuseAttestationParams)
			if err != nil {
				return err
			}
			attestations[0] = *aaoAttestation
			return nil
		})
	}

	for i, validator := range validators {
		innerGroup.Go(func() error {
			getValidatorAttestationParams := GetValidatorAttestationParams{
				Validator:      validator,
				Claim:          rewardClaim.RewardClaim,
				UserEthAddress: rewardClaim.RecipientEthAddress,
				Signature:      signature,
			}
			validatorAttestation, err := getValidatorAttestation(getValidatorAttestationParams)

			if err != nil {
				return err
			}
			attestations[i+offset] = *validatorAttestation
			return nil
		})
	}

	err := innerGroup.Wait()
	if err != nil {
		return nil, err
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
	tx := solana.NewTransactionBuilder()

	feePayer := transactionSender.GetFeePayer()
	tx.SetFeePayer(feePayer.PublicKey())

	for i, attestation := range attestations {
		instructionIndex := uint8(i * 2)
		submitAttestationSecpInstruction := secp256k1.NewSecp256k1Instruction(
			attestation.EthAddress,
			attestation.Message,
			attestation.Signature,
			instructionIndex,
		).Build()
		submitAttestationInstruction := reward_manager.NewSubmitAttestationInstruction(
			rewardClaim.RewardID,
			rewardClaim.Specifier,
			attestation.EthAddress,
			rewardManagerClient.GetProgramStateAccount(),
			feePayer.PublicKey(),
		).Build()
		tx.AddInstruction(submitAttestationSecpInstruction)
		tx.AddInstruction(submitAttestationInstruction)
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
		preTx := tx
		preTx.WithOpt(solana.TransactionAddressTables(addressLookupTables))
		err = transactionSender.AddPriorityFees(ctx, preTx, spl.AddPriorityFeesParams{Percentile: 99, Multiplier: 1})
		if err != nil {
			return nil, err
		}
		err = transactionSender.AddComputeBudgetLimit(ctx, preTx, spl.AddComputeBudgetLimitParams{Padding: 1000, Multiplier: 1.2})
		if err != nil {
			return nil, err
		}
		preTxBuilt, err := preTx.Build()
		if err != nil {
			return nil, err
		}

		preTxBinary, err := preTxBuilt.MarshalBinary()
		if err != nil {
			return nil, err
		}

		// Check to see if there's room for the evaluate instruction.
		// If not, send the attestations in a separate transaction.
		estimatedEvaluateInstructionSize := 205
		threshold := spl.MAX_TRANSACTION_SIZE - estimatedEvaluateInstructionSize
		if len(preTxBinary) > threshold {
			sig, err := transactionSender.SendTransactionWithRetries(ctx, preTx, rpc.CommitmentConfirmed, rpc.TransactionOpts{})
			if err != nil {
				return nil, err
			}
			txSignatures = append(txSignatures, *sig)
			tx = solana.NewTransactionBuilder()
		}
	}

	state, err := rewardManagerClient.GetProgramState(ctx)
	if err != nil {
		return nil, err
	}
	evaluateAttestationInstruction := reward_manager.NewEvaluateAttestationInstruction(
		rewardClaim.RewardID,
		rewardClaim.Specifier,
		common.HexToAddress(rewardClaim.RecipientEthAddress),
		rewardClaim.Amount*1e8, // Convert to wAUDIO wei
		common.HexToAddress(rewardClaim.AntiAbuseOracleEthAddress),
		rewardManagerClient.GetProgramStateAccount(),
		state.TokenAccount,
		rewardClaim.UserBank,
		feePayer.PublicKey(),
	).Build()
	tx.AddInstruction(evaluateAttestationInstruction)

	tx.WithOpt(solana.TransactionAddressTables(addressLookupTables))
	err = transactionSender.AddComputeBudgetLimit(ctx, tx, spl.AddComputeBudgetLimitParams{Padding: 1000, Multiplier: 1.2})
	if err != nil {
		return nil, err
	}
	err = transactionSender.AddPriorityFees(ctx, tx, spl.AddPriorityFeesParams{Percentile: 99, Multiplier: 1})
	if err != nil {
		return nil, err
	}

	sig, err := transactionSender.SendTransactionWithRetries(ctx, tx, rpc.CommitmentConfirmed, rpc.TransactionOpts{})
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
	validatorsNeeded := int(rewardManagerStateData.MinVotes) - len(existingValidatorOwners)
	selectedValidators, err := getValidators(validators, validatorsNeeded, existingValidatorOwners)
	if err != nil {
		return nil, err
	}
	attestations, err := fetchAttestations(
		ctx,
		rewardClaim,
		selectedValidators,
		antiAbuseOracle,
		signature,
		hasAntiAbuseOracleAttestation,
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
