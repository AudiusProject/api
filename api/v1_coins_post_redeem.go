package api

import (
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"api.audius.co/config"
	"api.audius.co/solana/spl"
	"api.audius.co/solana/spl/programs/reward_manager"
	"api.audius.co/utils"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	oap_common "github.com/OpenAudio/go-openaudio/pkg/common"
	oap_rewards "github.com/OpenAudio/go-openaudio/pkg/rewards"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gagliardetto/solana-go"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type CoinRewardParams struct {
	Mint   string
	Amount uint64
}

type RewardCodeRow struct {
	Mint          string `db:"mint"`
	RewardAddress string `db:"reward_address"`
	Amount        uint64 `db:"amount"`
	IsUsed        bool   `db:"is_used"`
}

func (app *ApiServer) v1CoinsPostRedeem(c *fiber.Ctx) error {
	// #region Validate Params
	if config.Cfg.LaunchpadDeterministicSecret == "" {
		return fiber.NewError(fiber.StatusInternalServerError, "Claim authority base is not configured")
	}

	mintString := c.Params("mint")
	if mintString == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mint is required",
		})
	}

	// Read optional code from route params
	redeemCode := c.Params("code")

	myId := app.getMyId(c)
	if myId == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "user_id is required",
		})
	}
	// #endregion Validate Params
	// #region Init State

	mint, err := solana.PublicKeyFromBase58(mintString)
	if err != nil {
		return fmt.Errorf("Mint is invalid: %s", mintString)
	}

	// Derive claim authority key for mint
	claimAuthorityPublicKey, claimAuthorityPrivKeyString, err := utils.DeriveEthAddressForMint(
		[]byte("claimAuthority"),
		config.Cfg.LaunchpadDeterministicSecret,
		mint,
	)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to derive Ethereum key: "+err.Error())
	}

	// Convert the private key to the format expected by the SDK
	claimAuthorityKey, err := oap_common.EthToEthKey(claimAuthorityPrivKeyString)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to convert private key: "+err.Error())
	}

	if err != nil {
		return fmt.Errorf("failed to get claim authority: %w", err)
	}

	// Lookup user
	var userWalletAddress string
	var userHandle string
	err = app.pool.QueryRow(c.Context(), `SELECT wallet, handle FROM users WHERE user_id = @user_id`, pgx.NamedArgs{
		"user_id": myId,
	}).Scan(&userWalletAddress, &userHandle)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusInternalServerError, "Failed to get user wallet")
		}
		return err
	}

	// Get reward manager state for the given mint
	// and init a new client for it
	var rewardStateAddress string
	var minVotes int
	err = app.pool.QueryRow(c.Context(), `SELECT reward_manager_state, min_votes FROM sol_reward_manager_inits WHERE mint = @mint`, pgx.NamedArgs{
		"mint": mintString,
	}).Scan(&rewardStateAddress, &minVotes)
	if err != nil {
		return fmt.Errorf("failed to get reward manager state: %w", err)
	}
	rewardManagerPubkey, err := solana.PublicKeyFromBase58(rewardStateAddress)
	if err != nil {
		return fmt.Errorf("failed to get init pubkey for reward manager state address: %w", err)
	}

	println(fmt.Sprintf("rewardManagerPubkey: %s", rewardManagerPubkey))

	rewardManagerClient, err := reward_manager.NewRewardManagerClient(
		app.solanaRpcClient,
		app.solanaConfig.RewardManagerProgramID,
		rewardManagerPubkey,
		app.solanaConfig.RewardManagerLookupTable,
		app.logger,
	)

	if err != nil {
		return fmt.Errorf("failed to create reward manager client: %w", err)
	}

	// Ensure user bank exists
	bankAccount, err := app.claimableTokensClient.GetOrCreateUserBank(
		c.Context(),
		common.HexToAddress(userWalletAddress),
		mint,
	)
	if err != nil {
		return fmt.Errorf("failed to get or create user bank: %w", err)
	}

	// #endregion Init State/Userbank
	// #region Burn Code
	amount := uint64(0)
	rewardAddress := ""
	specifier := ""
	if redeemCode != "" {
		// Read and burn code in one go
		// Use CTE to capture old is_used value before updating
		sql := `WITH old_row AS (
			SELECT is_used, reward_address, amount
			FROM reward_codes
			WHERE code = @code
			AND mint = @mint
		)
		UPDATE reward_codes
		SET is_used = true
		FROM old_row
		WHERE reward_codes.code = @code
		RETURNING
			reward_codes.reward_address,
			reward_codes.amount,
			old_row.is_used AS was_already_used`
		rows, err := app.writePool.Query(c.Context(), sql, pgx.NamedArgs{
			"code": redeemCode,
			"mint": mintString,
		})
		if err != nil {
			return err
		}

		type RewardCodeBurnResultRow struct {
			RewardAddress  string `db:"reward_address"`
			Amount         uint64 `db:"amount"`
			WasAlreadyUsed bool   `db:"was_already_used"`
		}
		rewardCode, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[RewardCodeBurnResultRow])
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "invalid",
				})
			}
			return err
		}

		// TODO: remove
		// if rewardCode.WasAlreadyUsed {
		// 	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
		// 		"error": "used",
		// 	})
		// }

		amount = rewardCode.Amount
		rewardAddress = rewardCode.RewardAddress
		specifier = redeemCode
	} else {
		// No code provided, attempt to redeem the generic amount for the mint
		// TODO: Handle 1 token case where code isn't provided
		// Probably want to check challenge disbursements and bail early if user has already claimed
		amount = 1
		rewardAddress = ""
		// Specifier defaults to user ID if no code is provided
		// TODO: Make sure this logic is correct for the various use cases
		// PLAN: Ticker is the code here. Move this if/else up further, we will use the same burn logic for both.
		specifier = strconv.Itoa(int(myId))
	}

	// #endregion Burn Code
	// #region Build Claim
	rewardClaim := oap_rewards.RewardClaim{
		RecipientEthAddress: userWalletAddress,
		Amount:              amount,
		RewardID:            redeemCode,
		RewardAddress:       rewardAddress,
		Specifier:           specifier,
		ClaimAuthority:      claimAuthorityPublicKey,
	}

	// attestationSignature, err := oap_rewards.SignClaim(rewardClaim, claimAuthorityKey)
	// if err != nil {
	// 	return fmt.Errorf("failed to sign for claimAuthority: %w", err)
	// }

	result := ClaimResult{
		ChallengeID: "code",
		Specifier:   specifier,
		Amount:      amount,
		Signatures:  []solana.Signature{},
		Error:       "",
	}

	decoratedRewardClaim := RewardClaim{
		RewardClaim:   rewardClaim,
		Handle:        userHandle,
		UserBank:      *bankAccount,
		TokenDecimals: 1e9, // TODO: get from DB
	}

	claimMessage, err := rewardClaim.Compile()

	attestations := make([]SenderAttestation, 0)
	for _, validator := range app.validators.GetNodes() {
		if len(attestations) >= minVotes {
			break
		}
		oap := sdk.NewOpenAudioSDK(validator.Endpoint)
		oap.SetPrivKey(claimAuthorityKey)

		response, err := oap.Rewards.GetRewardAttestation(c.Context(), &v1.GetRewardAttestationRequest{
			EthRecipientAddress: userWalletAddress,
			Amount:              1000,
			RewardAddress:       rewardAddress,
			RewardId:            redeemCode,
			Specifier:           specifier,
			ClaimAuthority:      oap.Address(),
		})

		// TODO: better
		if err != nil {
			println(fmt.Sprintf("failed to get reward attestation: %v", err))
			continue
		}

		// Pad the start if there's a missing leading zero
		signature := response.Attestation
		if len(signature)%2 == 1 {
			signature = "0" + signature
		}
		signatureBytes, err := hex.DecodeString(strings.TrimPrefix(signature, "0x"))
		if err != nil {
			println(fmt.Sprintf("failed to decode signature: %v", err))
			continue
		}

		attestation := SenderAttestation{
			EthAddress: common.HexToAddress(response.Owner),
			Message:    claimMessage,
			Signature:  signatureBytes,
		}
		attestations = append(attestations, attestation)
	}

	// TODO: Probably needs to move into rewardManagerClient
	// (deriveATA function accepting mint?)
	seeds := make([][]byte, 1)
	seeds[0] = rewardManagerClient.GetProgramStateAccount().Bytes()[0:32]
	ataAuthority, _, err := solana.FindProgramAddress(seeds, config.Cfg.SolanaConfig.RewardManagerProgramID)
	if err != nil {
		return fmt.Errorf("failed to derive ATA authority: %w", err)
	}

	// Derive Associated Token Account address
	// Seeds: [owner, token_program_id, mint]
	// TODO: Remove hardcoded associated token program ID
	associatedTokenProgramID := solana.MustPublicKeyFromBase58("ATokenGPvbdGVxr1b2hvZbsiqW5xWH25efTNsLJA8knL")
	ata, _, err := solana.FindProgramAddress(
		[][]byte{
			ataAuthority.Bytes(),
			solana.TokenProgramID.Bytes(),
			mint.Bytes(),
		},
		associatedTokenProgramID,
	)
	if err != nil {
		return fmt.Errorf("failed to derive ATA: %w", err)
	}

	// #endregion Build Claim
	// #region Send Tx
	// Build and send solana transactions
	signatures, err := sendRewardClaimTransactions(
		c.Context(),
		rewardManagerClient,
		app.transactionSender,
		decoratedRewardClaim,
		attestations,
		&ata,
	)

	if err != nil {
		var instrErr *spl.InstructionError
		if errors.As(err, &instrErr) {
			app.logger.Error("failed to claim challenge reward. transaction failed to send.",
				zap.String("handle", userHandle),
				zap.String("rewardId", "code"),
				zap.String("specifier", specifier),
				zap.String("transaction", instrErr.EncodedTransaction),
				zap.String("customError", reward_manager.RewardManagerError(instrErr.Code).String()),
				zap.Error(err),
			)
		} else {
			app.logger.Error("failed to claim challenge reward.",
				zap.String("handle", userHandle),
				zap.String("rewardId", "code"),
				zap.String("specifier", specifier),
				zap.Error(err),
			)
		}
		result.Error = err.Error()
	}

	result.Signatures = signatures

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"data": []ClaimResult{result},
	})
}
