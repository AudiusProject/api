package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type VerifyTokenQueryParams struct {
	Token string `query:"token" validate:"required"`
}

func (app *ApiServer) v1UsersVerifyToken(c *fiber.Ctx) error {
	queryParams := VerifyTokenQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}

	// 1. Break JWT into parts
	tokenParts := strings.Split(queryParams.Token, ".")
	if len(tokenParts) != 3 {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid JWT token format")
	}

	base64Header := tokenParts[0]
	base64Payload := tokenParts[1]
	base64Signature := tokenParts[2]

	// 2. Decode the signature
	// Add padding if needed for base64 decoding
	paddedSignature := base64Signature
	if len(paddedSignature)%4 != 0 {
		paddedSignature += strings.Repeat("=", 4-len(paddedSignature)%4)
	}

	signatureDecoded, err := base64.URLEncoding.DecodeString(paddedSignature)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "The JWT signature could not be decoded")
	}

	// Convert decoded bytes to string (hex string) and then decode hex to bytes
	signatureHex := string(signatureDecoded)
	signatureBytes := common.FromHex(signatureHex)

	// 3. Recover wallet from signature using ethereum message recovery
	message := fmt.Sprintf("%s.%s", base64Header, base64Payload)

	// Create the ethereum signed message format
	encodedToRecover := []byte(message)
	prefixedMessage := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(encodedToRecover), encodedToRecover))
	finalHash := crypto.Keccak256Hash(prefixedMessage)

	if len(signatureBytes) != 65 {
		return fiber.NewError(fiber.StatusBadRequest, "The JWT signature was incorrectly signed")
	}
	// Ethereum signatures are 65 bytes long, with the last byte being the recovery ID.
	// The recovery ID is 0 or 1, and is used to determine which public key was used to sign the message.
	// The recovery ID is 27 or 28, and is used to determine which public key was used to sign the message.
	// We need to subtract 27 from the recovery ID to get the correct public key.
	if signatureBytes[64] >= 27 {
		signatureBytes[64] -= 27
	}

	publicKey, err := crypto.SigToPub(finalHash.Bytes(), signatureBytes)
	if err != nil {
		return fiber.NewError(fiber.StatusNotFound, "The JWT signature is invalid - wallet could not be recovered")
	}

	recoveredAddr := crypto.PubkeyToAddress(*publicKey)
	walletLower := strings.ToLower(recoveredAddr.Hex())

	// 4. Check that user from payload matches the user from the wallet from the signature
	paddedPayload := base64Payload
	if len(paddedPayload)%4 != 0 {
		paddedPayload += strings.Repeat("=", 4-len(paddedPayload)%4)
	}

	stringifiedPayload, err := base64.URLEncoding.DecodeString(paddedPayload)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JWT payload could not be decoded")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(stringifiedPayload, &payload); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "JWT payload could not be unmarshalled")
	}

	// Convert numeric fields to strings to match legacy behavior
	if iat, exists := payload["iat"]; exists {
		if iatNum, ok := iat.(float64); ok {
			payload["iat"] = fmt.Sprintf("%.0f", iatNum)
		}
	}

	// Get user ID from JWT payload
	userIdInterface, exists := payload["userId"]
	if !exists {
		return fiber.NewError(fiber.StatusBadRequest, "JWT payload missing userId field")
	}

	userIdStr, ok := userIdInterface.(string)
	if !ok {
		return fiber.NewError(fiber.StatusBadRequest, "JWT payload userId must be a string")
	}

	jwtUserId, err := trashid.DecodeHashId(userIdStr)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "Invalid JWT payload userId")
	}

	// Get user ID associated with recovered wallet
	walletUserId, err := app.queries.GetUserForWallet(c.Context(), walletLower)
	if err != nil {
		if err == pgx.ErrNoRows {
			return fiber.NewError(fiber.StatusNotFound, "The JWT signature is invalid - invalid wallet")
		}
		return err
	}

	// Check if wallet user matches JWT user or if wallet user is a manager
	if int32(walletUserId) != int32(jwtUserId) {
		// Check if the signing user is a manager of the JWT user
		isManager, err := app.isActiveManager(c.Context(), int32(jwtUserId), int32(walletUserId))
		if err != nil {
			return err
		}

		if !isManager {
			return fiber.NewError(fiber.StatusForbidden, "The JWT signature is invalid - the wallet does not match the user")
		}
	}

	// 5. Send back the decoded payload
	return c.JSON(fiber.Map{
		"data": payload,
	})
}

func (app *ApiServer) isActiveManager(ctx context.Context, userId int32, managerUserId int32) (bool, error) {
	grants, err := app.queries.GetGrantsForUserId(ctx, dbv1.GetGrantsForUserIdParams{
		UserID:     userId,
		IsRevoked:  false,
		IsApproved: pgtype.Bool{Bool: true, Valid: true},
	})
	if err != nil {
		return false, err
	}

	for _, grant := range grants {
		if int32(grant.GranteeUserID) == managerUserId {
			return true, nil
		}
	}

	return false, nil
}
