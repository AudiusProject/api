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

	// Use shared function to create reward code
	rewardAddress, err := app.createRewardCode(context.Background(), code, req.Mint, req.Amount, "Launchpad")
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to create reward code: "+err.Error())
	}

	// Insert the reward code into the database
	sql := `
		INSERT INTO reward_codes (code, mint, reward_address, amount, remaining_uses, signature)
		VALUES (@code, @mint, @reward_address, @amount, 1, @signature)
		RETURNING code, mint, reward_address, amount
	`

	rows, err := app.writePool.Query(context.Background(), sql, pgx.NamedArgs{
		"code":           code,
		"mint":           req.Mint,
		"reward_address": rewardAddress,
		"amount":         req.Amount,
		"signature":      codeSignature,
	})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Database error: "+err.Error())
	}

	response, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[CreateRewardCodeResponse])
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to read response: "+err.Error())
	}

	return c.Status(fiber.StatusCreated).JSON(response)
}

// createRewardCode creates or reuses a reward pool and returns the reward address.
// This is shared business logic used by both v1CreateRewardCode and prize claim flow.
func (app *ApiServer) createRewardCode(ctx context.Context, code, mint string, amount int64, rewardName string) (string, error) {
	var rewardAddress string

	// Only create reward pool if deterministic secret is configured
	if app.config.LaunchpadDeterministicSecret != "" {
		// Check for existing reward address for this mint (reuse pattern)
		var existingRewardAddress string
		err := app.pool.QueryRow(ctx, `
			SELECT reward_address FROM reward_codes 
			WHERE mint = $1 AND reward_address IS NOT NULL AND reward_address != ''
			LIMIT 1
		`, mint).Scan(&existingRewardAddress)

		if err == nil && existingRewardAddress != "" {
			// Reuse existing reward pool
			rewardAddress = existingRewardAddress
		} else {
			// Create new reward pool
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
				return "", fmt.Errorf("failed to derive Ethereum key: %w", err)
			}

			// Convert the private key to the format expected by the SDK
			privateKey, err := common.EthToEthKey(claimAuthorityPrivateKey)
			if err != nil {
				return "", fmt.Errorf("failed to convert private key: %w", err)
			}

			// Create OpenAudio SDK instance and set the private key
			oap := sdk.NewOpenAudioSDK(app.config.AudiusdURL)
			oap.SetPrivKey(privateKey)

			// Get current chain status to calculate deadline
			statusResp, err := oap.Core.GetStatus(ctx, connect.NewRequest(&v1.GetStatusRequest{}))
			if err != nil {
				return "", fmt.Errorf("failed to get chain status: %w", err)
			}

			currentHeight := statusResp.Msg.ChainInfo.CurrentHeight
			deadline := currentHeight + 100
			rewardID := code

			reward, err := oap.Rewards.CreateReward(ctx, &v1.CreateReward{
				RewardId: rewardID,
				Name:     fmt.Sprintf("%s Reward %s", rewardName, code),
				Amount:   uint64(amount),
				ClaimAuthorities: []*v1.ClaimAuthority{
					{Address: claimAuthority, Name: rewardName},
				},
				DeadlineBlockHeight: deadline,
			})
			if err != nil {
				return "", fmt.Errorf("failed to create reward pool: %w", err)
			}

			rewardAddress = reward.Address
		}
	} else {
		rewardAddress = ""
	}

	return rewardAddress, nil
}
