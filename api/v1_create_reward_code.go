package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"

	"api.audius.co/config"
	"api.audius.co/utils"
	"connectrpc.com/connect"
	v1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	"github.com/OpenAudio/go-openaudio/pkg/common"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
	"github.com/gagliardetto/solana-go"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

const (
	signedMessage = "code"
	codeLength    = 6
	codeChars     = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
)

type CreateRewardCodeRequest struct {
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

func verifySignature(signatureBase64 string, authorizedPubKey string) (bool, error) {
	// Decode the signature from base64
	signatureBytes, err := base64.StdEncoding.DecodeString(signatureBase64)
	if err != nil {
		return false, err
	}

	// Parse the expected public key
	expectedPubKey, err := solana.PublicKeyFromBase58(authorizedPubKey)
	if err != nil {
		return false, err
	}

	// Verify the signature
	message := []byte(signedMessage)
	valid := ed25519.Verify(expectedPubKey[:], message, signatureBytes)

	return valid, nil
}

func verifySignatureAgainstKeys(signatureBase64 string, authorizedKeys []string) (string, error) {
	for _, key := range authorizedKeys {
		valid, err := verifySignature(signatureBase64, key)
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

	// Verify the signature against authorized keys from config
	_, err := verifySignatureAgainstKeys(req.Signature, config.Cfg.RewardCodeAuthorizedKeys)
	if err != nil {
		return fiber.NewError(fiber.StatusForbidden, "Unauthorized: "+err.Error())
	}

	// Parse the mint public key
	mintPubKey, err := solana.PublicKeyFromBase58(req.Mint)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid mint address: "+err.Error())
	}

	// Derive Ethereum address and private key from deterministic secret
	if config.Cfg.LaunchpadDeterministicSecret == "" {
		return fiber.NewError(fiber.StatusInternalServerError, "Launchpad deterministic secret not configured")
	}

	ethAddress, ethPrivateKey, err := utils.DeriveEthAddressForMint(
		[]byte("launchpad_reward_code"),
		config.Cfg.LaunchpadDeterministicSecret,
		mintPubKey,
	)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to derive Ethereum key: "+err.Error())
	}

	// Convert the private key to the format expected by the SDK
	privateKey, err := common.EthToEthKey(ethPrivateKey)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to convert private key: "+err.Error())
	}

	// Create OpenAudio SDK instance and set the private key
	oap := sdk.NewOpenAudioSDK(config.Cfg.AudiusdURL)
	oap.SetPrivKey(privateKey)

	// Get current chain status to calculate deadline
	statusResp, err := oap.Core.GetStatus(context.Background(), connect.NewRequest(&v1.GetStatusRequest{}))
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to get chain status: "+err.Error())
	}

	currentHeight := statusResp.Msg.ChainInfo.CurrentHeight
	deadline := currentHeight + 1000 // Set deadline 1000 blocks in the future

	// Generate a code for the reward ID
	code, err := generateCode()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to generate code: "+err.Error())
	}

	rewardID := fmt.Sprintf("reward_%s", code)

	// Create the reward pool
	reward, err := oap.Rewards.CreateReward(context.Background(), &v1.CreateReward{
		RewardId: rewardID,
		Name:     fmt.Sprintf("Launchpad Reward %s", code),
		Amount:   uint64(req.Amount),
		ClaimAuthorities: []*v1.ClaimAuthority{
			{Address: ethAddress, Name: "Launchpad"},
		},
		DeadlineBlockHeight: deadline,
	})
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to create reward pool: "+err.Error())
	}

	// Insert the reward code into the database
	sql := `
		INSERT INTO reward_codes (code, mint, reward_address, amount, is_used)
		VALUES (@code, @mint, @reward_address, @amount, false)
		RETURNING code, mint, reward_address, amount
	`

	rows, err := app.writePool.Query(context.Background(), sql, pgx.NamedArgs{
		"code":           code,
		"mint":           req.Mint,
		"reward_address": reward.Address,
		"amount":         req.Amount,
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
