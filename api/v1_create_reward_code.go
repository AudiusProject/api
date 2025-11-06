package api

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"math/big"

	"api.audius.co/config"
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
	matchedKey, err := verifySignatureAgainstKeys(req.Signature, config.Cfg.RewardCodeAuthorizedKeys)
	if err != nil {
		return fiber.NewError(fiber.StatusForbidden, "Unauthorized: "+err.Error())
	}

	code, err := generateCode()
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to generate code: "+err.Error())
	}

	sql := `
		INSERT INTO reward_codes (code, mint, reward_address, amount, is_used)
		VALUES (@code, @mint, @reward_address, @amount, false)
		RETURNING code, mint, reward_address, amount
	`

	rows, err := app.writePool.Query(context.Background(), sql, pgx.NamedArgs{
		"code":           code,
		"mint":           req.Mint,
		"reward_address": matchedKey,
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
