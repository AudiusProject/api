package api

import (
	"crypto/ecdsa"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"api.audius.co/config"
	"api.audius.co/solana/spl/programs/reward_manager"
	oap_rewards "github.com/AudiusProject/audiusd/pkg/rewards"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gagliardetto/solana-go"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func getAttestationSignatureForRewardClaimAuthority(rewardClaim oap_rewards.RewardClaim, claimAuthorityKey *ecdsa.PrivateKey) (string, error) {
	claimData, err := rewardClaim.Compile()
	if err != nil {
		return "", fmt.Errorf("failed to get attestation bytes: %w", err)
	}

	hash := crypto.Keccak256(claimData)

	signatureBytes, err := crypto.Sign(hash, claimAuthorityKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign hash: %w", err)
	}

	return "0x" + hex.EncodeToString(signatureBytes), nil
}

// TODO: In ac scripts, this uses a "domain" of "claimAuthority"
func getClaimAuthority(mint solana.PublicKey) (*ecdsa.PrivateKey, error) {
	return nil, nil
}

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
	mintString := c.Params("mint")
	if mintString == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Mint parameter is required")
	}

	// Read optional code from route params
	redeemCode := c.Params("code")

	myId := app.getMyId(c)
	if myId == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "Missing user ID")
	}

	var userWalletAddress string
	err := app.pool.QueryRow(c.Context(), `SELECT wallet FROM users WHERE user_id = @user_id`, pgx.NamedArgs{
		"user_id": myId,
	}).Scan(&userWalletAddress)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusInternalServerError, "Failed to get user wallet")
		}
		return err
	}

	mint, err := solana.PublicKeyFromBase58(mintString)
	if err != nil {
		return fmt.Errorf("Mint is invalid: %s", mintString)
	}

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

	bankAccount, err := app.claimableTokensClient.GetOrCreateUserBank(
		c.Context(),
		common.HexToAddress(userWalletAddress),
		mint,
	)
	if err != nil {
		return fmt.Errorf("failed to get or create user bank: %w", err)
	}

	amount := uint64(0)
	rewardAddress := ""
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
			reward_codes.is_used,
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
				return fiber.NewError(fiber.StatusNotFound, "Reward code not found")
			}
			return err
		}

		if rewardCode.WasAlreadyUsed {
			return fiber.NewError(fiber.StatusBadRequest, "Reward code already used")
		}

		amount = rewardCode.Amount
		rewardAddress = rewardCode.RewardAddress
	} else {
		// TODO: Handle 1 token case where code isn't provided
		// Probably want to check challenge disbursements and bail early if user has already claimed
	}

	// Specifier defaults to user ID if no code is provided
	// TODO: Make sure this logic is correct for the various use cases
	specifier := strconv.Itoa(int(myId))
	if redeemCode != "" {
		specifier = redeemCode
	}

	claimAuthorityKey, err := getClaimAuthority(mint)
	if err != nil {
		return fmt.Errorf("failed to get claim authority: %w", err)
	}
	claimAuthorityPublicKey := crypto.PubkeyToAddress(claimAuthorityKey.PublicKey)

	rewardClaim := oap_rewards.RewardClaim{
		RecipientEthAddress: userWalletAddress,
		Amount:              amount,
		RewardID:            "a", // TODO: Either fetch with oap or should come from row
		RewardAddress:       rewardAddress,
		Specifier:           specifier,
		ClaimAuthority:      claimAuthorityPublicKey.Hex(),
	}

	attestationSignature, err := getAttestationSignatureForRewardClaimAuthority(rewardClaim, claimAuthorityKey)
	if err != nil {
		return fmt.Errorf("failed to get claim authority attestation signature: %w", err)
	}

	decoratedRewardClaim := RewardClaim{
		RewardClaim: rewardClaim,
		Handle:      "", // TODO: Also fetch with wallet
		UserBank:    *bankAccount,
	}

	// Reusing generic claim flow but we don't care about AAO attestation
	attestations, err := fetchAttestations(
		c.Context(),
		decoratedRewardClaim,
		app.validators,
		[]string{},
		config.Node{
			DelegateOwnerWallet: "",
			Endpoint:            "",
		},
		attestationSignature,
		true,
		minVotes,
	)
	if err != nil {
		return fmt.Errorf("failed to fetch attestations: %w", err)
	}

	// Build and send solana transactions
	signatures, err := sendRewardClaimTransactions(
		c.Context(),
		rewardManagerClient,
		app.transactionSender,
		decoratedRewardClaim,
		attestations,
	)

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"data": signatures,
	})
}
