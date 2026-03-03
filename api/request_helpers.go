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

// getApiSigner extracts a signer from the Authorization header.
// Supports Bearer token and Basic auth. In both cases, the credential is checked
// as an api_access_key first; for Basic auth, a raw private key hex is also accepted.
func (app *ApiServer) getApiSigner(c *fiber.Ctx) (*Signer, error) {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return nil, fmt.Errorf("missing Authorization header")
	}

	// Bearer: extract token and look up as api_access_key
	if strings.HasPrefix(authHeader, "Bearer ") {
		token := strings.TrimSpace(strings.TrimPrefix(authHeader, "Bearer "))
		if token == "" {
			return nil, fmt.Errorf("Bearer token is empty")
		}
		if app.writePool != nil {
			if signer := app.getSignerFromApiAccessKey(c.Context(), token); signer != nil {
				return signer, nil
			}
		}
		// If authMiddleware already validated a JWT and set authedWallet,
		// use AudiusApiSecret to sign on behalf of the authenticated user.
		if wallet, _ := c.Locals("authedWallet").(string); wallet != "" && app.config.AudiusApiSecret != "" {
			apiSecret, err := crypto.HexToECDSA(strings.TrimPrefix(app.config.AudiusApiSecret, "0x"))
			if err == nil {
				return &Signer{
					Address:    strings.ToLower(crypto.PubkeyToAddress(apiSecret.PublicKey).Hex()),
					PrivateKey: apiSecret,
				}, nil
			}
		}
		return nil, fmt.Errorf("invalid Bearer token")
	}

	// Basic: decode credentials and use password as api_access_key or private key
	if !strings.HasPrefix(authHeader, "Basic ") {
		return nil, fmt.Errorf("Authorization must be Bearer or Basic")
	}

	encodedCreds := strings.TrimPrefix(authHeader, "Basic ")
	decodedBytes, err := base64.StdEncoding.DecodeString(encodedCreds)
	if err != nil {
		return nil, fmt.Errorf("failed to decode Basic Auth credentials: %w", err)
	}

	creds := string(decodedBytes)
	parts := strings.SplitN(creds, ":", 2)
	var password string
	if len(parts) == 2 {
		password = strings.TrimSpace(parts[1])
	} else {
		password = strings.TrimSpace(creds)
	}
	password = strings.TrimPrefix(password, "0x")

	// Try api_access_key lookup (also try raw encoded value for clients that send it un-encoded)
	if app.writePool != nil {
		for _, candidate := range []string{password, encodedCreds} {
			if candidate == "" {
				continue
			}
			if signer := app.getSignerFromApiAccessKey(c.Context(), candidate); signer != nil {
				return signer, nil
			}
		}
	}
	if password == "" {
		return nil, fmt.Errorf("invalid Basic Auth format")
	}

	// Fallback: treat password as raw private key hex
	privateKey, err := crypto.HexToECDSA(password)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}
	address := crypto.PubkeyToAddress(privateKey.PublicKey)
	return &Signer{
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
			Address:    strings.ToLower(hit.ApiKey),
			PrivateKey: privateKey,
		}
	}

	var parentApiKey, apiSecret string
	err := app.writePool.QueryRow(ctx, `
		SELECT aak.api_key, ak.api_secret
		FROM api_access_keys aak
		JOIN api_keys ak ON LOWER(ak.api_key) = LOWER(aak.api_key)
		WHERE aak.api_access_key = $1 AND aak.is_active = true
	`, apiAccessKey).Scan(&parentApiKey, &apiSecret)
	if err == pgx.ErrNoRows || err != nil || apiSecret == "" {
		return nil
	}

	parentApiKeyLower := strings.ToLower(parentApiKey)
	app.apiAccessKeySignerCache.Set(apiAccessKey, apiAccessKeySignerEntry{
		ApiKey:    parentApiKeyLower,
		ApiSecret: apiSecret,
	})

	privateKey, err := crypto.HexToECDSA(strings.TrimPrefix(apiSecret, "0x"))
	if err != nil {
		return nil
	}
	return &Signer{
		Address:    parentApiKeyLower,
		PrivateKey: privateKey,
	}
}
