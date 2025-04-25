package api

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/api/spl"
	"bridgerton.audius.co/api/spl/programs/claimable_tokens"
	"bridgerton.audius.co/api/spl/programs/reward_manager"
	"bridgerton.audius.co/api/spl/programs/secp256k1"
	"bridgerton.audius.co/config"
	"bridgerton.audius.co/trashid"
	"github.com/AudiusProject/audiusd/pkg/rewards"
	"github.com/gagliardetto/solana-go"
	"github.com/gagliardetto/solana-go/rpc"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/sync/errgroup"
)

type SenderAttestation struct {
	Signature string
	Address   string
}

const (
	validatorAttestationPath       = "/core/rewards/attestation"
	antiAbuseOracleAttestationPath = "/attestation"
)

var rewardManagerStateData *reward_manager.RewardManagerState = nil
var rewardManagerAddressLookupTable *spl.AddressLookupTable
var antiAbuseOracleMap map[string]string = make(map[string]string)

type GetAntiAbuseOracleAttestationParams struct {
	ChallengeID             string
	Specifier               string
	Amount                  uint64
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
		ChallengeID: args.ChallengeID,
		Specifier:   args.Specifier,
		Amount:      args.Amount,
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
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to get oracle attestation from "+args.AntiAbuseOracleEndpoint+". Error "+resp.Status+": "+string(body))
	}

	parsedBody := AntiAbuseOracleAttestationResponseBody{}
	err = json.Unmarshal(body, &parsedBody)
	if err != nil {
		return nil, err
	}
	address, exists := antiAbuseOracleMap[args.AntiAbuseOracleEndpoint]
	if !exists {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to find AAO address for "+args.AntiAbuseOracleEndpoint)
	}
	attestation := SenderAttestation{
		Address:   address,
		Signature: parsedBody.Result,
	}
	return &attestation, nil
}

// Selects three uniquely owned, healthy validators at random
// TODO: add health checks?
func getValidators(validators []config.Node, count int, excludedOperators []string) ([]string, error) {
	shuffled := slices.Clone(validators)
	rand.Shuffle(min(len(validators), count), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	selected := make([]string, 0)
	for _, node := range shuffled {
		if !slices.Contains(excludedOperators, node.OperatorEthAddress) {
			selected = append(selected, node.Endpoint)
			excludedOperators = append(excludedOperators, node.OperatorEthAddress)
		}
	}
	return selected, nil
}

type GetValidatorAttestationParams struct {
	Validator              string
	ChallengeID            string
	Specifier              string
	Amount                 uint64
	UserEthAddress         string
	AntiAbuseOracleAddress string
	Signature              string
}

type ValidatorAttestationResponseBody struct {
	Attestation string `json:"attestation"`
	Owner       string `json:"owner"`
}

// Gets a reward claim attestation from a validator.
func getValidatorAttestation(args GetValidatorAttestationParams) (*SenderAttestation, error) {
	query := url.Values{}
	query.Add("reward_id", args.ChallengeID)
	query.Add("specifier", args.Specifier)
	query.Add("eth_recipient_address", args.UserEthAddress)
	query.Add("oracle_address", args.AntiAbuseOracleAddress)
	query.Add("amount", strconv.FormatUint(args.Amount, 10))
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
		return nil, fiber.NewError(fiber.StatusInternalServerError, "Failed to get validator attestation from "+args.Validator+". Error "+resp.Status+": "+string(body))
	}
	attestation := ValidatorAttestationResponseBody{}
	err = json.Unmarshal(body, &attestation)
	if err != nil {
		return nil, err
	}
	return &SenderAttestation{Address: attestation.Owner, Signature: attestation.Attestation}, nil
}

// Gets reward claim attestations from AAO and Validators in parallel.
func fetchAttestations(
	ctx context.Context,
	rewardClaim rewards.RewardClaim,
	handle string,
	validators []string,
	antiAbuseOracleEndpoint string,
	antiAbuseOracleAddress string,
	signature string,
	hasAntiAbuseOracleAttestation bool,
) (*SenderAttestation, []SenderAttestation, error) {
	var aaoAttestation *SenderAttestation
	validatorAttestations := make([]SenderAttestation, len(validators))

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	innerGroup, _ := errgroup.WithContext(ctx)

	if !hasAntiAbuseOracleAttestation {
		innerGroup.Go(func() error {
			getAntiAbuseAttestationParams := GetAntiAbuseOracleAttestationParams{
				ChallengeID:             rewardClaim.RewardID,
				Specifier:               rewardClaim.Specifier,
				Amount:                  rewardClaim.Amount,
				Handle:                  handle,
				AntiAbuseOracleEndpoint: antiAbuseOracleEndpoint,
			}
			res, err := getAntiAbuseOracleAttestation(getAntiAbuseAttestationParams)
			if err == nil {
				aaoAttestation = res
			}
			return err
		})
	}

	for i, validator := range validators {
		innerGroup.Go(func() error {
			getValidatorAttestationParams := GetValidatorAttestationParams{
				Validator:              validator,
				ChallengeID:            rewardClaim.RewardID,
				Specifier:              rewardClaim.Specifier,
				Amount:                 rewardClaim.Amount,
				UserEthAddress:         rewardClaim.RecipientEthAddress,
				AntiAbuseOracleAddress: antiAbuseOracleAddress,
				Signature:              signature,
			}
			validatorAttestation, err := getValidatorAttestation(getValidatorAttestationParams)
			if err == nil {
				validatorAttestations[i] = *validatorAttestation
			}
			return err
		})
	}

	err := innerGroup.Wait()
	if err != nil {
		return nil, nil, err
	}
	return aaoAttestation, validatorAttestations, nil
}

// Builds a Solana transaction to claim a reward from the attestations.
func buildRewardClaimTransaction(
	rewardClaim rewards.RewardClaim,
	aaoAttestation *SenderAttestation,
	validatorAttestations []SenderAttestation,
	solanaConfig config.SolanaConfig,
	client *rpc.Client,
) (*solana.Transaction, error) {
	feePayer := solanaConfig.FeePayers[rand.IntN(len(solanaConfig.FeePayers))]

	bankAccount, err := claimable_tokens.DeriveUserBankAccount(solanaConfig.MintAudio, rewardClaim.RecipientEthAddress)
	if err != nil {
		return nil, err
	}

	// Build the transaction
	tx := solana.TransactionBuilder{}

	if aaoAttestation != nil {
		aaoAddressBytes, err := hex.DecodeString(strings.TrimPrefix(aaoAttestation.Address, "0x"))
		if err != nil {
			return nil, err
		}

		// Pad the start if there's a missing leading zero
		if len(aaoAttestation.Signature)%2 == 1 {
			aaoAttestation.Signature = "0" + aaoAttestation.Signature
		}
		aaoSignatureBytes, err := hex.DecodeString(aaoAttestation.Signature)
		if err != nil {
			return nil, err
		}

		aaoClaim := rewardClaim
		// AAO claims don't have the oracle appended
		aaoClaim.AntiAbuseOracleEthAddress = ""
		aaoAttestationBytes, err := aaoClaim.Compile()
		if err != nil {
			return nil, err
		}

		// Add AAO attestation instructions
		aaoSubmitAttestationSecpInstruction := secp256k1.NewSecp256k1Instruction(
			aaoAddressBytes,
			aaoAttestationBytes,
			aaoSignatureBytes,
			0,
		).Build()
		aaoSubmitAttestationInstruction := reward_manager.NewSubmitAttestationInstruction(
			rewardClaim.RewardID,
			rewardClaim.Specifier,
			aaoAttestation.Address,
			solanaConfig.RewardManagerState,
			feePayer.PublicKey(),
		).Build()
		tx.AddInstruction(aaoSubmitAttestationSecpInstruction)
		tx.AddInstruction(aaoSubmitAttestationInstruction)
	}

	// Add Validator attestation instructions
	attestationBytes, err := rewardClaim.Compile()
	if err != nil {
		return nil, err
	}
	for i, attestation := range validatorAttestations {
		instructionIndex := uint8(i*2 + 2)

		senderEthAddressBytes, err := hex.DecodeString(strings.TrimPrefix(attestation.Address, "0x"))
		if err != nil {
			return nil, err
		}

		// Pad the start if there's a missing leading zero
		if len(attestation.Signature)%2 == 1 {
			attestation.Signature = "0" + attestation.Signature
		}
		signatureBytes, err := hex.DecodeString(strings.TrimPrefix(attestation.Signature, "0x"))
		if err != nil {
			return nil, err
		}

		submitAttestationSecpInstruction := secp256k1.NewSecp256k1Instruction(
			senderEthAddressBytes,
			attestationBytes,
			signatureBytes,
			instructionIndex,
		).Build()
		submitAttestationInstruction := reward_manager.NewSubmitAttestationInstruction(
			rewardClaim.RewardID,
			rewardClaim.Specifier,
			attestation.Address,
			solanaConfig.RewardManagerState,
			feePayer.PublicKey(),
		).Build()
		tx.AddInstruction(submitAttestationSecpInstruction)
		tx.AddInstruction(submitAttestationInstruction)
	}

	// Add evaluate instructions
	evaluateAttestationInstruction := reward_manager.NewEvaluateAttestationInstruction(
		rewardClaim.RewardID,
		rewardClaim.Specifier,
		rewardClaim.RecipientEthAddress,
		rewardClaim.Amount*1e8, // Convert to wAUDIO wei
		rewardClaim.AntiAbuseOracleEthAddress,
		solanaConfig.RewardManagerState,
		rewardManagerStateData.TokenAccount,
		bankAccount,
		feePayer.PublicKey(),
	).Build()
	tx.AddInstruction(evaluateAttestationInstruction)

	tx.SetFeePayer(feePayer.PublicKey())
	addressLookupTables := map[solana.PublicKey]solana.PublicKeySlice{
		solanaConfig.RewardManagerLookupTable: rewardManagerAddressLookupTable.Addresses,
	}
	tx.WithOpt(solana.TransactionAddressTables(addressLookupTables))

	recent, err := client.GetLatestBlockhash(context.TODO(), rpc.CommitmentFinalized)
	if err != nil {
		return nil, err
	}
	tx.SetRecentBlockHash(recent.Value.Blockhash)

	transaction, err := tx.Build()
	if err != nil {
		return nil, err
	}
	return transaction, nil
}

type RelayTransactionRequest struct {
	Transaction string `json:"transaction"`
}
type RelayTransactionResponse struct {
	Signature string `json:"signature"`
}

// Relays transactions to the Solana Relay plugin.
// TODO: Move Solana Relay into Bridge
func relayTransaction(relay string, transaction *solana.Transaction) (string, error) {
	encoded, err := transaction.ToBase64()
	if err != nil {
		return "", err
	}

	jsonData, err := json.Marshal(RelayTransactionRequest{Transaction: encoded})
	if err != nil {
		return "", err
	}

	resp, err := http.Post(relay, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != 200 {
		return "", fiber.NewError(fiber.StatusInternalServerError, "Failed to relay transaction: "+encoded+". Error "+resp.Status+": "+string(body))
	}

	parsedResp := RelayTransactionResponse{}
	err = json.Unmarshal(body, &parsedResp)
	if err != nil {
		return "", err
	}

	return parsedResp.Signature, nil
}

// Claims an individual reward.
func claimReward(ctx context.Context, row dbv1.GetUndisbursedChallengesRow, antiAbuseOracleAddress string, antiAbuseOracleEndpoint string, rewardAttester *rewards.RewardAttester, solanaConfig config.SolanaConfig, validators []config.Node) (string, error) {
	feePayer := solanaConfig.FeePayers[rand.IntN(len(solanaConfig.FeePayers))]

	rewardClaim := rewards.RewardClaim{
		RewardID:                  row.ChallengeID,
		Amount:                    uint64(5), // TODO: Change me!
		Specifier:                 row.Specifier,
		RecipientEthAddress:       row.Wallet.String,
		AntiAbuseOracleEthAddress: antiAbuseOracleAddress,
	}

	handle := row.Handle.String

	// Get the RewardManagerState
	client := rpc.New(solanaConfig.RpcProviders[0])
	if rewardManagerStateData == nil {
		rewardManagerStateData = &reward_manager.RewardManagerState{}
		err := client.GetAccountDataBorshInto(ctx, solanaConfig.RewardManagerState, rewardManagerStateData)
		if err != nil {
			return "", err
		}
	}

	// Get the address lookup table
	if rewardManagerAddressLookupTable == nil {
		rewardManagerAddressLookupTable = &spl.AddressLookupTable{}
		err := client.GetAccountDataInto(ctx, solanaConfig.RewardManagerLookupTable, rewardManagerAddressLookupTable)
		if err != nil {
			return "", err
		}
	}

	// Get current claim state to do minimum work to claim
	disbursementId := rewardClaim.RewardID + ":" + rewardClaim.Specifier
	authority, _, err := reward_manager.DeriveAuthorityAccount(reward_manager.ProgramID, solanaConfig.RewardManagerState)
	if err != nil {
		return "", err
	}
	attestationsAccountAddress, _, err := reward_manager.DeriveAttestationsAccount(reward_manager.ProgramID, authority, disbursementId)
	if err != nil {
		return "", err
	}
	attestationsData := reward_manager.AttestationsAccountData{}
	err = client.GetAccountDataInto(ctx, attestationsAccountAddress, &attestationsData)
	if err != nil {
		return "", err
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
	_, signature, err := rewardAttester.Attest(rewardClaim)
	if err != nil {
		return "", err
	}

	// Fetch AAO and  validator attestations
	selectedValidators, err := getValidators(validators, int(rewardManagerStateData.MinVotes), existingValidatorOwners)
	if err != nil {
		return "", err
	}
	aaoAttestation, validatorAttestations, err := fetchAttestations(
		ctx,
		rewardClaim,
		handle,
		selectedValidators,
		antiAbuseOracleEndpoint,
		antiAbuseOracleAddress,
		signature,
		hasAntiAbuseOracleAttestation,
	)
	if err != nil {
		return "", err
	}

	// Build transaction
	transaction, err := buildRewardClaimTransaction(
		rewardClaim,
		aaoAttestation,
		validatorAttestations,
		solanaConfig,
		client,
	)
	if err != nil {
		return "", err
	}

	transaction.Sign(func(key solana.PublicKey) *solana.PrivateKey {
		return &feePayer.PrivateKey
	})

	// Send transaction
	txSig, err := relayTransaction(solanaConfig.SolanaRelay, transaction)

	if err != nil {
		return "", err
	}

	return txSig, nil
}

type HealthCheckResponse struct {
	AntiAbuseWalletPubkey string
}

// Selects a healthy Anti Abuse Oracle and gets its address.
// TODO: Implement AAO in bridge and use config
func getAntiAbuseOracle(antiAbuseOracleEndpoints []string) (endpoint string, address string, err error) {
	oracleEndpoint := antiAbuseOracleEndpoints[rand.IntN(len(antiAbuseOracleEndpoints))]

	if value, exists := antiAbuseOracleMap[oracleEndpoint]; exists {
		return oracleEndpoint, value, nil
	}

	resp, err := http.Get(oracleEndpoint + "/health_check")
	if err != nil {
		return "", "", err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}
	if resp.StatusCode != 200 {
		return "", "", fiber.NewError(fiber.StatusInternalServerError, "Failed to get oracle from "+oracleEndpoint+". Error "+resp.Status+": "+string(body))
	}
	health := &HealthCheckResponse{}
	err = json.Unmarshal(body, health)
	if err != nil {
		return "", "", err
	}

	antiAbuseOracleMap[oracleEndpoint] = health.AntiAbuseWalletPubkey
	return oracleEndpoint, health.AntiAbuseWalletPubkey, nil
}

// Claims all the filtered undisbursed rewards for a user.
func (api *ApiServer) v1ClaimRewards(c *fiber.Ctx) error {
	challengeId := c.Query("challenge_id")
	specifier := c.Query("specifier")
	hashId := c.Query("user_id")

	if hashId == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing user ID")
	}

	userId, err := trashid.DecodeHashId(hashId)
	if err != nil {
		return err
	}

	undisbursedRows, err := api.queries.GetUndisbursedChallenges(
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

	antiAbuseOracleEndpoint, antiAbuseOracleAddress, err := getAntiAbuseOracle(api.antiAbuseOracles)

	if err != nil {
		return err
	}

	signatures := make([]string, len(undisbursedRows))
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	g, ctx := errgroup.WithContext(ctx)
	for i, row := range undisbursedRows {
		g.Go(func() error {
			sig, err := claimReward(
				ctx,
				row,
				antiAbuseOracleAddress,
				antiAbuseOracleEndpoint,
				&api.rewardAttester,
				api.solanaConfig,
				api.validators,
			)
			signatures[i] = sig
			return err
		})
	}
	err = g.Wait()
	if err != nil {
		return err
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"data": signatures,
	})
}
