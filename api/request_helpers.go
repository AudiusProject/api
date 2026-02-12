package api

import (
	"context"
	"crypto/ecdsa"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

// apiAccessKeySignerEntry caches the result of api_access_key -> (api_key, api_secret) lookup
type apiAccessKeySignerEntry struct {
	ApiKey    string
	ApiSecret string
}

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

// getApiSigner extracts a signer from the Basic Auth header.
// If the password is an api_access_key, looks up api_keys for the api_secret (private key hex).
// Otherwise treats the password as a raw private key hex.
func (app *ApiServer) getApiSigner(c *fiber.Ctx) (*Signer, error) {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing Authorization header")
	}

	if !strings.HasPrefix(authHeader, "Basic ") {
		return nil, fmt.Errorf("Authorization header is not Basic Auth")
	}

	encodedCreds := strings.TrimPrefix(authHeader, "Basic ")
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedCreds)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Basic Auth credentials: %w", err)
	}

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

	// Branch A: Try api_access_key lookup (password)
	if app.writePool != nil && privateKeyHex != "" {
		if signer := app.getSignerFromApiAccessKey(c.Context(), privateKeyHex); signer != nil {
			return signer, nil
		}
	}

	// Branch B: Treat password as direct private key hex
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	return &Signer{
		UserId:     userId,
		Address:    address.Hex(),
		PrivateKey: privateKey,
	}, nil
}

// getSignerFromApiAccessKey looks up api_access_keys and api_keys to build a Signer.
func (app *ApiServer) getSignerFromApiAccessKey(ctx context.Context, apiAccessKey string) *Signer {
	if hit, ok := app.apiAccessKeySignerCache.Get(apiAccessKey); ok {
		privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(hit.ApiSecret, "0x"))
		if err != nil {
			return nil
		}
		return &Signer{
			Address:    hit.ApiKey,
			PrivateKey: privateKey,
		}
	}

	var parentApiKey, apiSecret string
	err := app.writePool.QueryRow(ctx, `
		SELECT aak.api_key, ak.api_secret
		FROM api_access_keys aak
		JOIN api_keys ak ON ak.api_key = aak.api_key
		WHERE aak.api_access_key = $1 AND aak.is_active = true
	`, apiAccessKey).Scan(&parentApiKey, &apiSecret)
	if err == pgx.ErrNoRows || err != nil || apiSecret == "" {
		return nil
	}

	app.apiAccessKeySignerCache.Set(apiAccessKey, apiAccessKeySignerEntry{
		ApiKey:    parentApiKey,
		ApiSecret: apiSecret,
	})

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(apiSecret, "0x"))
	if err != nil {
		return nil
	}
	return &Signer{
		Address:    parentApiKey,
		PrivateKey: privateKey,
	}
}
