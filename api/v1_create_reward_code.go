package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
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
	signedAuthMessage = "code"
	codeLength        = 10
	codeChars         = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

type CreateRewardCodeRequest struct {
	Timestamp int64  `json:"timestamp" validate:"omitempty,min=1"`
	Signature string `json:"signature" validate:"required"`
	Mint      string `json:"mint" validate:"required"`
	Amount    int64  `json:"amount" validate:"required,min=1"`
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

func verifySignature(signatureBase58 string, message string, authorizedPubKey string) (bool, error) {
	// Decode the signature from base58
	signatureBytes, err := base58.Decode(signatureBase58)
	if err != nil {
		return false, err
	}

	// Parse the expected public key
	expectedPubKey, err := solana.PublicKeyFromBase58(authorizedPubKey)
	if err != nil {
		return false, err
	}

	// Verify the signature
	messageBytes := []byte(message)
	valid := ed25519.Verify(expectedPubKey[:], messageBytes, signatureBytes)
	return valid, nil
}

func verifySignatureAgainstKeys(signatureBase58 string, message string, authorizedKeys []string) (string, error) {
	for _, key := range authorizedKeys {
		valid, err := verifySignature(signatureBase58, message, key)
		if err != nil {
			// If there's an error parsing the key or signature, continue to next key
			continue
		}
		if valid {
			// Found a matching key
			return key, nil
		}
	}
	// No matching key found
	return "", errors.New("unauthorized")
}

func (app *ApiServer) v1CreateRewardCode(c *fiber.Ctx) error {
	var req CreateRewardCodeRequest
	if err := app.ParseAndValidateBody(c, &req); err != nil {
		return err
	}
	var message = signedAuthMessage
	signatureIsSingleUse := req.Timestamp > 0
	if signatureIsSingleUse {
		message = fmt.Sprintf("%d", req.Timestamp)
		var signatureUsed bool
		// Check if signature already used
		if err := app.writePool.QueryRow(context.Background(), `
		SELECT EXISTS(SELECT 1 FROM reward_codes WHERE signature = $1)
	`, req.Signature).Scan(&signatureUsed); err != nil {
			app.logger.Error("Failed to query for existing verified signature", zap.Error(err))
			return fiber.NewError(fiber.StatusInternalServerError)
		} else if signatureUsed {
			return fiber.NewError(fiber.StatusBadRequest, "Duplicate signature")
		}

		timestamp := time.UnixMilli(req.Timestamp)
		// Allow drift of timestamp
		if time.Since(timestamp).Abs() > (12 * time.Hour) {
			return fiber.NewError(fiber.StatusBadRequest, "Timestamp out of range")
		}
	}

	_, err := verifySignatureAgainstKeys(req.Signature, message, app.config.RewardCodeAuthorizedKeys)
	if err != nil {
		return fiber.NewError(fiber.StatusForbidden, "Unauthorized: "+err.Error())
	}

	// Generate a code
	code, err := generateCode()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to generate code: "+err.Error())
	}

	var codeSignature string
	if signatureIsSingleUse {
		codeSignature = req.Signature
	} else {
		codeSignature = ""
	}

	// Use shared function to create reward code and insert into database
	rewardAddress, err := app.createAndInsertRewardCode(context.Background(), code, req.Mint, req.Amount, "Launchpad", codeSignature)
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

// createRewardCode creates or reuses a reward pool and returns the reward address.
// This is shared business logic used by both v1CreateRewardCode and prize claim flow.
func (app *ApiServer) createRewardCode(ctx context.Context, code, mint string, amount int64, rewardName string) (string, error) {
	app.logger.Info("createRewardCode: Starting",
		zap.String("code", code),
		zap.String("mint", mint),
		zap.Int64("amount", amount),
		zap.String("reward_name", rewardName),
		zap.Bool("has_deterministic_secret", app.config.LaunchpadDeterministicSecret != ""),
		zap.String("audiusd_url", app.config.AudiusdURL))

	var rewardAddress string

	// Only create reward pool if deterministic secret is configured
	if app.config.LaunchpadDeterministicSecret != "" {
		app.logger.Info("createRewardCode: Deterministic secret configured, checking for existing reward pool",
			zap.String("mint", mint))
		// Check for existing reward address for this mint (reuse pattern)
		var existingRewardAddress string
		err := app.pool.QueryRow(ctx, `
			SELECT reward_address FROM reward_codes 
			WHERE mint = $1 AND reward_address IS NOT NULL AND reward_address != ''
			LIMIT 1
		`, mint).Scan(&existingRewardAddress)

		if err == nil && existingRewardAddress != "" {
			// Reuse existing reward pool
			app.logger.Info("createRewardCode: Reusing existing reward pool",
				zap.String("mint", mint),
				zap.String("reward_address", existingRewardAddress))
			rewardAddress = existingRewardAddress
		} else {
			if err != nil && err != pgx.ErrNoRows {
				app.logger.Warn("createRewardCode: Error checking for existing reward pool, will create new",
					zap.String("mint", mint),
					zap.Error(err))
			} else {
				app.logger.Info("createRewardCode: No existing reward pool found, creating new",
					zap.String("mint", mint))
			}

			// Create new reward pool
			app.logger.Info("createRewardCode: Parsing mint public key",
				zap.String("mint", mint))
			mintPubKey, err := solana.PublicKeyFromBase58(mint)
			if err != nil {
				app.logger.Error("createRewardCode: Invalid mint address",
					zap.String("mint", mint),
					zap.Error(err))
				return "", fmt.Errorf("invalid mint address: %w", err)
			}

			app.logger.Info("createRewardCode: Deriving Ethereum address for mint",
				zap.String("mint", mint))
			claimAuthority, claimAuthorityPrivateKey, err := utils.DeriveEthAddressForMint(
				[]byte("claimAuthority"),
				app.config.LaunchpadDeterministicSecret,
				mintPubKey,
			)
			if err != nil {
				app.logger.Error("createRewardCode: Failed to derive Ethereum key",
					zap.String("mint", mint),
					zap.Error(err))
				return "", fmt.Errorf("failed to derive Ethereum key: %w", err)
			}
			app.logger.Info("createRewardCode: Ethereum address derived",
				zap.String("claim_authority", claimAuthority),
				zap.String("mint", mint))

			// Convert the private key to the format expected by the SDK
			app.logger.Info("createRewardCode: Converting private key format")
			privateKey, err := common.EthToEthKey(claimAuthorityPrivateKey)
			if err != nil {
				app.logger.Error("createRewardCode: Failed to convert private key",
					zap.Error(err))
				return "", fmt.Errorf("failed to convert private key: %w", err)
			}

			// Create OpenAudio SDK instance and set the private key
			app.logger.Info("createRewardCode: Creating OpenAudio SDK instance",
				zap.String("audiusd_url", app.config.AudiusdURL))
			oap := sdk.NewOpenAudioSDK(app.config.AudiusdURL)
			oap.SetPrivKey(privateKey)

			// Get current chain status to calculate deadline
			app.logger.Info("createRewardCode: Getting chain status")
			statusResp, err := oap.Core.GetStatus(ctx, connect.NewRequest(&v1.GetStatusRequest{}))
			if err != nil {
				app.logger.Error("createRewardCode: Failed to get chain status",
					zap.String("audiusd_url", app.config.AudiusdURL),
					zap.Error(err))
				return "", fmt.Errorf("failed to get chain status: %w", err)
			}

			currentHeight := statusResp.Msg.ChainInfo.CurrentHeight
			deadline := currentHeight + 100
			rewardID := code

			// Convert from whole YAK (as stored in database) to smallest units for OpenAudio SDK
			// reward_codes.amount stores whole YAK, but OpenAudio SDK expects smallest units (9 decimals)
			amountInSmallestUnits := amount * 1000000000
			app.logger.Info("createRewardCode: Creating reward pool",
				zap.String("reward_id", rewardID),
				zap.String("name", fmt.Sprintf("%s Reward %s", rewardName, code)),
				zap.Int64("amount_whole_yak", amount),
				zap.Uint64("amount_smallest_units", uint64(amountInSmallestUnits)),
				zap.String("claim_authority", claimAuthority),
				zap.Int64("deadline", deadline))

			reward, err := oap.Rewards.CreateReward(ctx, &v1.CreateReward{
				RewardId: rewardID,
				Name:     fmt.Sprintf("%s Reward %s", rewardName, code),
				Amount:   uint64(amountInSmallestUnits),
				ClaimAuthorities: []*v1.ClaimAuthority{
					{Address: claimAuthority, Name: rewardName},
				},
				DeadlineBlockHeight: deadline,
			})
			if err != nil {
				app.logger.Error("createRewardCode: Failed to create reward pool via OpenAudio SDK",
					zap.String("reward_id", rewardID),
					zap.String("audiusd_url", app.config.AudiusdURL),
					zap.Error(err))
				return "", fmt.Errorf("failed to create reward pool: %w", err)
			}

			rewardAddress = reward.Address
			app.logger.Info("createRewardCode: Reward pool created successfully",
				zap.String("reward_address", rewardAddress),
				zap.String("reward_id", rewardID))
		}
	} else {
		app.logger.Info("createRewardCode: No deterministic secret configured, skipping reward pool creation",
			zap.String("mint", mint))
		rewardAddress = ""
	}

	app.logger.Info("createRewardCode: Completed",
		zap.String("code", code),
		zap.String("reward_address", rewardAddress),
		zap.Bool("has_reward_address", rewardAddress != ""))
	return rewardAddress, nil
}
