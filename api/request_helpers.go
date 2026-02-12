package api

import (
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgtype"
)

// Signer holds the address, public key, and private key for signing transactions
type Signer struct {
	UserId     int
	Address    string
	PrivateKey *ecdsa.PrivateKey
}

func getOptionalBool(c *fiber.Ctx, key string) (pgtype.Bool, error) {
	if valueStr := c.Query(key); valueStr != "" {
		parsed, err := strconv.ParseBool(c.Query(key))
		if err != nil {
			return pgtype.Bool{}, err
		}
		return pgtype.Bool{Bool: parsed, Valid: true}, nil
	}
	return pgtype.Bool{}, nil
}

// getApiSigner extracts a private key from the Basic Auth header and returns a Signer
// The Basic Auth username is ignored, and the password should contain the private key hex string (without 0x prefix)
func (app *ApiServer) getApiSigner(c *fiber.Ctx) (*Signer, error) {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing Authorization header")
	}

	// Check if it's a Basic Auth header
	if !strings.HasPrefix(authHeader, "Basic ") {
		return nil, fmt.Errorf("Authorization header is not Basic Auth")
	}

	// Decode the base64 encoded credentials
	encodedCreds := strings.TrimPrefix(authHeader, "Basic ")
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedCreds)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Basic Auth credentials: %w", err)
	}

	// Split username:password
	creds := string(decodedBytes)
	parts := strings.SplitN(creds, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid Basic Auth format")
	}

	userId, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid userId: %w", err)
	}

	// The private key is in the password field (parts[1])
	privateKeyHex := strings.TrimPrefix(parts[1], "0x")

	// Parse the private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	// Derive the public key and address from the private key
	address := crypto.PubkeyToAddress(privateKey.PublicKey)

	return &Signer{
		UserId:     userId,
		Address:    address.Hex(),
		PrivateKey: privateKey,
	}, nil
}
