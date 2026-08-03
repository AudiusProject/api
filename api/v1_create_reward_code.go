package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"math/big"
	"strconv"
	"time"

	"api.audius.co/utils"
	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/OpenAudio/go-openaudio/pkg/common"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
	"github.com/gagliardetto/solana-go"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/mr-tron/base58"
	"go.uber.org/zap"
)

const (
	codeLength = 10
	codeChars  = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

	// rewardPoolDeadlineWindow is the number of blocks ahead of the
	// current height at which we set the deadline_block_height on
	// cometbft tx envelopes that this server originates
	// (CreateRewardPool, CreateReward). Cheap to keep generous: the
	// deadline only bounds how stale a single signed envelope can sit
	// before the validator rejects it.
	rewardPoolDeadlineWindow = 100
)

// CreateRewardCodeRequest carries a signed Solana transaction containing a
// single Memo Program instruction whose data is a millisecond-epoch timestamp
// (as a UTF-8 decimal string). The transaction's fee-payer is the signer and
// must appear in RewardCodeAuthorizedKeys. The transaction is never submitted
// on-chain — it's used purely as a signing envelope so every wallet flow
// (hot wallets and hardware wallets like Ledger) can authenticate uniformly.
type CreateRewardCodeRequest struct {
	SignedTransaction string `json:"signed_transaction" validate:"required"`
	Mint              string `json:"mint" validate:"required"`
	Amount            int64  `json:"amount" validate:"required,min=1"`
}

type CreateRewardCodeResponse struct {
	Code          string `json:"code"`
	Mint          string `json:"mint"`
	RewardAddress string `json:"reward_address"`
	Amount        int64  `json:"amount"`
}

func generateCode() (string, error) {
	result := make([]byte, codeLength)
	charsLen := big.NewInt(int64(len(codeChars)))

	for i := 0; i < codeLength; i++ {
		num, err := rand.Int(rand.Reader, charsLen)
		if err != nil {
			return "", err
		}
		result[i] = codeChars[num.Int64()]
	}

	return string(result), nil
}

// extractMemoTimestamp finds the first Memo Program instruction in the
// transaction and parses its data as a millisecond-epoch timestamp. Returns
// an error if no memo instruction is present or its data isn't a valid
// integer.
func extractMemoTimestamp(tx *solana.Transaction) (int64, error) {
	for _, ix := range tx.Message.Instructions {
		if int(ix.ProgramIDIndex) >= len(tx.Message.AccountKeys) {
			continue
		}
		if !tx.Message.AccountKeys[ix.ProgramIDIndex].Equals(solana.MemoProgramID) {
			continue
		}
		ts, err := strconv.ParseInt(string(ix.Data), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("memo data is not a valid timestamp: %w", err)
		}
		return ts, nil
	}
	return 0, fmt.Errorf("no memo instruction found")
}

func (app *ApiServer) v1CreateRewardCode(c *fiber.Ctx) error {
	var req CreateRewardCodeRequest
	if err := app.ParseAndValidateBody(c, &req); err != nil {
		return err
	}

	// Decode the signed transaction
	tx, err := solana.TransactionFromBase64(req.SignedTransaction)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid transaction: "+err.Error())
	}

	// Must carry exactly one signature (the authorizing signer / fee payer)
	if len(tx.Signatures) != 1 || len(tx.Message.AccountKeys) == 0 {
		return fiber.NewError(fiber.StatusBadRequest, "Transaction must have exactly one signer")
	}

	// Cryptographically verify the signature against the message bytes
	if err := tx.VerifySignatures(); err != nil {
		return fiber.NewError(fiber.StatusForbidden, "Invalid signature: "+err.Error())
	}

	// Fee payer (account index 0) is the authorizing signer
	signer := tx.Message.AccountKeys[0].String()
	signerAuthorized := false
	for _, key := range app.config.RewardCodeAuthorizedKeys {
		if key == signer {
			signerAuthorized = true
			break
		}
	}
	if !signerAuthorized {
		return fiber.NewError(fiber.StatusForbidden, "Unauthorized: signer "+signer+" not in allowlist")
	}

	// Extract and validate the timestamp embedded in the memo instruction
	timestamp, err := extractMemoTimestamp(tx)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, err.Error())
	}
	if time.Since(time.UnixMilli(timestamp)).Abs() > (12 * time.Hour) {
		return fiber.NewError(fiber.StatusBadRequest, "Timestamp out of range")
	}

	// The transaction signature uniquely identifies this signing event.
	// We use it as the replay-prevention key.
	txSig := tx.Signatures[0].String()

	var signatureUsed bool
	if err := app.writePool.QueryRow(context.Background(), `
		SELECT EXISTS(SELECT 1 FROM reward_codes WHERE signature = $1)
	`, txSig).Scan(&signatureUsed); err != nil {
		app.logger.Error("Failed to query for existing signature", zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError)
	}
	if signatureUsed {
		return fiber.NewError(fiber.StatusBadRequest, "Duplicate signature")
	}

	// Generate a code
	code, err := generateCode()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to generate code: "+err.Error())
	}

	rewardAddress, err := app.createAndInsertRewardCode(context.Background(), code, req.Mint, req.Amount, "Launchpad", txSig)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to create reward code: "+err.Error())
	}

	response := CreateRewardCodeResponse{
		Code:          code,
		Mint:          req.Mint,
		RewardAddress: rewardAddress,
		Amount:        req.Amount,
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// createAndInsertRewardCode creates or reuses a reward pool, inserts the reward code into the database,
// and returns the reward address. This is shared business logic used by both v1CreateRewardCode and prize claim flow.
func (app *ApiServer) createAndInsertRewardCode(ctx context.Context, code, mint string, amount int64, rewardName, signature string) (string, error) {
	app.logger.Info("createAndInsertRewardCode: Starting",
		zap.String("code", code),
		zap.String("mint", mint),
		zap.Int64("amount", amount),
		zap.String("reward_name", rewardName),
		zap.Bool("has_signature", signature != ""))

	// First create the reward code
	app.logger.Info("createAndInsertRewardCode: Creating reward code",
		zap.String("code", code),
		zap.String("mint", mint))
	rewardAddress, err := app.createRewardCode(ctx, code, mint, amount, rewardName)
	if err != nil {
		app.logger.Error("createAndInsertRewardCode: Failed to create reward code",
			zap.String("code", code),
			zap.String("mint", mint),
			zap.String("reward_name", rewardName),
			zap.Error(err))
		return "", err
	}
	app.logger.Info("createAndInsertRewardCode: Reward code created",
		zap.String("code", code),
		zap.String("reward_address", rewardAddress),
		zap.Bool("has_reward_address", rewardAddress != ""))

	// Insert the reward code into the database
	app.logger.Info("createAndInsertRewardCode: Inserting into database",
		zap.String("code", code),
		zap.String("mint", mint),
		zap.String("reward_address", rewardAddress))
	sql := `
		INSERT INTO reward_codes (code, mint, reward_address, amount, remaining_uses, signature)
		VALUES (@code, @mint, @reward_address, @amount, 1, @signature)
		ON CONFLICT (code) DO NOTHING
	`

	_, err = app.writePool.Exec(ctx, sql, pgx.NamedArgs{
		"code":           code,
		"mint":           mint,
		"reward_address": rewardAddress,
		"amount":         amount,
		"signature":      signature,
	})
	if err != nil {
		app.logger.Error("createAndInsertRewardCode: Database insert failed",
			zap.String("code", code),
			zap.String("mint", mint),
			zap.Error(err))
		return "", fmt.Errorf("database error: %w", err)
	}

	app.logger.Info("createAndInsertRewardCode: Successfully completed",
		zap.String("code", code),
		zap.String("reward_address", rewardAddress))
	return rewardAddress, nil
}

// createRewardCode creates a cometbft reward bound to the launchpad mint's
// pool and returns the reward address. Idempotent on the pool (a pool that
// already exists is reused; only the very first reward for a brand-new
// mint triggers CreateRewardPool).
//
// Three keys are involved:
//   - The per-mint claim authority eth key (secp256k1, from
//     DeriveEthAddressForMint). Signs the cometbft envelope and is the
//     pool's sole initial authority.
//   - The RM ed25519 keypair (from DeriveRewardManagerKeypair). Same
//     keypair the solana-relay used to init the Solana reward manager
//     state account; its public key IS the rewards_manager_pubkey.
//     Signs the CreateRewardPool envelope's rm_owner_signature, which
//     proves possession of the RM keypair and prevents pool-creation
//     frontrunning.
//
// Both are derived from app.config.LaunchpadDeterministicSecret +
// the mint, so they're available everywhere the secret is configured.
// When the secret is empty, this function is a no-op and returns ""
// (matches existing behavior for dev environments without launchpad
// configuration).
func (app *ApiServer) createRewardCode(ctx context.Context, code, mint string, amount int64, rewardName string) (string, error) {
	app.logger.Info("createRewardCode: Starting",
		zap.String("code", code),
		zap.String("mint", mint),
		zap.Int64("amount", amount),
		zap.String("reward_name", rewardName),
		zap.Bool("has_deterministic_secret", app.config.LaunchpadDeterministicSecret != ""),
		zap.String("audiusd_url", app.config.AudiusdURL))

	if app.config.LaunchpadDeterministicSecret == "" {
		app.logger.Info("createRewardCode: Completed (no launchpad secret configured; reward pool skipped)",
			zap.String("code", code))
		return "", nil
	}

	mintPubKey, err := solana.PublicKeyFromBase58(mint)
	if err != nil {
		return "", fmt.Errorf("invalid mint address: %w", err)
	}

	claimAuthority, claimAuthorityPrivateKey, err := utils.DeriveEthAddressForMint(
		[]byte("claimAuthority"),
		app.config.LaunchpadDeterministicSecret,
		mintPubKey,
	)
	if err != nil {
		return "", fmt.Errorf("failed to derive eth claim-authority key: %w", err)
	}
	envelopeKey, err := common.EthToEthKey(claimAuthorityPrivateKey)
	if err != nil {
		return "", fmt.Errorf("failed to convert eth claim-authority key: %w", err)
	}

	// Derive the RM ed25519 keypair matching what the solana-relay used
	// to init the Solana reward manager state account. The base58-encoded
	// public key IS the rewards_manager_pubkey cometbft carries for this
	// mint's pool.
	rmKey := utils.DeriveRewardManagerKeypair(app.config.LaunchpadDeterministicSecret, mintPubKey)
	rewardsManagerPubkey := base58.Encode(rmKey.Public().(ed25519.PublicKey))

	oap := sdk.NewOpenAudioSDK(app.config.AudiusdURL)
	oap.SetPrivKey(envelopeKey)

	statusResp, err := oap.Core.GetStatus(ctx, connect.NewRequest(&v1.GetStatusRequest{}))
	if err != nil {
		return "", fmt.Errorf("failed to get chain status: %w", err)
	}
	deadline := statusResp.Msg.ChainInfo.CurrentHeight + rewardPoolDeadlineWindow

	// First reward against this mint? Create the pool. Pre-existing pool
	// is the common case (every subsequent reward for the same mint).
	if _, err := oap.Rewards.GetRewardPool(ctx, rewardsManagerPubkey); err != nil {
		if connect.CodeOf(err) != connect.CodeNotFound {
			return "", fmt.Errorf("failed to look up reward pool for RM %s: %w", rewardsManagerPubkey, err)
		}
		app.logger.Info("createRewardCode: Creating reward pool",
			zap.String("mint", mint),
			zap.String("rewards_manager_pubkey", rewardsManagerPubkey),
			zap.String("claim_authority", claimAuthority))
		if _, createErr := oap.Rewards.CreateRewardPool(ctx, &v1.CreateRewardPool{
			RewardsManagerPubkey: rewardsManagerPubkey,
			Authorities:          []string{claimAuthority},
		}, rmKey, deadline); createErr != nil {
			// Race window: two concurrent first-reward requests for the
			// same brand-new mint can both observe NotFound and both
			// submit CreateRewardPool. The second one will fail because
			// the pool now exists. Re-fetch and treat "pool exists" as
			// success — equivalent to having lost the race cleanly.
			// Anything else is a real error.
			if _, getErr := oap.Rewards.GetRewardPool(ctx, rewardsManagerPubkey); getErr != nil {
				return "", fmt.Errorf("failed to create reward pool: %w", createErr)
			}
			app.logger.Info("createRewardCode: Lost CreateRewardPool race; pool now exists",
				zap.String("rewards_manager_pubkey", rewardsManagerPubkey))
		}
	}

	reward, err := oap.Rewards.CreateReward(ctx, &v1.CreateReward{
		RewardId:             code,
		Name:                 fmt.Sprintf("Launchpad Reward %s", code),
		Amount:               uint64(amount),
		RewardsManagerPubkey: rewardsManagerPubkey,
	}, deadline)
	if err != nil {
		return "", fmt.Errorf("failed to create reward: %w", err)
	}

	app.logger.Info("createRewardCode: Completed",
		zap.String("code", code),
		zap.String("reward_address", reward.Address),
		zap.String("rewards_manager_pubkey", rewardsManagerPubkey))
	return reward.Address, nil
}
