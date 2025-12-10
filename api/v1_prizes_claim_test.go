package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestV1PrizesClaim(t *testing.T) {
	app := emptyTestApp(t)

	// Run migration to create claimed_prizes table
	ctx := context.Background()
	// Drop table if it exists to ensure clean state
	_, err := app.writePool.Exec(ctx, `DROP TABLE IF EXISTS claimed_prizes CASCADE`)
	require.NoError(t, err)

	_, err = app.writePool.Exec(ctx, `
		CREATE TABLE claimed_prizes (
			id SERIAL PRIMARY KEY,
			wallet VARCHAR NOT NULL,
			signature VARCHAR NOT NULL UNIQUE,
			mint VARCHAR NOT NULL,
			amount BIGINT NOT NULL,
			prize_id VARCHAR NOT NULL,
			prize_name VARCHAR NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX claimed_prizes_wallet_idx ON claimed_prizes (wallet);
		CREATE INDEX claimed_prizes_signature_idx ON claimed_prizes (signature);
		CREATE INDEX claimed_prizes_mint_idx ON claimed_prizes (mint);
	`)
	require.NoError(t, err)

	// Create artist_coins table for decimals lookup
	_, err = app.writePool.Exec(ctx, `DROP TABLE IF EXISTS artist_coins CASCADE`)
	require.NoError(t, err)

	_, err = app.writePool.Exec(ctx, `
		CREATE TABLE artist_coins (
			mint VARCHAR NOT NULL PRIMARY KEY,
			ticker VARCHAR NOT NULL,
			user_id INT NOT NULL,
			decimals INT NOT NULL,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
	`)
	require.NoError(t, err)

	// Ensure sol_token_account_balance_changes table exists
	_, err = app.writePool.Exec(ctx, `DROP TABLE IF EXISTS sol_token_account_balance_changes CASCADE`)
	require.NoError(t, err)

	_, err = app.writePool.Exec(ctx, `
		CREATE TABLE sol_token_account_balance_changes (
			signature VARCHAR NOT NULL,
			mint VARCHAR NOT NULL,
			owner VARCHAR NOT NULL,
			account VARCHAR NOT NULL,
			change BIGINT NOT NULL,
			balance BIGINT NOT NULL,
			slot BIGINT NOT NULL,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			block_timestamp TIMESTAMP NOT NULL,
			fee_payer VARCHAR,
			PRIMARY KEY (signature, mint, account)
		);
	`)
	require.NoError(t, err)

	const (
		yakMintAddress = "ZDaUDL4XFdEct7UgeztrFQAptsvh4ZdhyZDZ1RpxYAK"
		yakSpinAmount  = 250000000000 // 250 YAK with 9 decimals
		validWallet    = "HLnpSz9h2S4hiLQ43rnSD9XkcUThA7B8hQMKmDaiTLcC"
		validSignature = "valid_signature_123"
		otherWallet    = "DDT15s6MMNxE4jkyGN46wNYqrgLWofT6WAvWtjYYrCUq"
	)

	// Insert test coin data
	_, err = app.writePool.Exec(ctx, `
		INSERT INTO artist_coins (mint, ticker, user_id, decimals)
		VALUES ($1, 'YAK', 1, 9)
		ON CONFLICT (mint) DO NOTHING
	`, yakMintAddress)
	require.NoError(t, err)

	// Create prizes table
	_, err = app.writePool.Exec(ctx, `DROP TABLE IF EXISTS prizes CASCADE`)
	require.NoError(t, err)

	_, err = app.writePool.Exec(ctx, `
		CREATE TABLE prizes (
			id SERIAL PRIMARY KEY,
			prize_id VARCHAR NOT NULL UNIQUE,
			name VARCHAR NOT NULL,
			description TEXT,
			weight INT NOT NULL DEFAULT 1,
			is_active BOOLEAN NOT NULL DEFAULT true,
			metadata JSONB,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX prizes_active_idx ON prizes (is_active);
	`)
	require.NoError(t, err)

	// Insert test prizes
	_, err = app.writePool.Exec(ctx, `
		INSERT INTO prizes (prize_id, name, weight, is_active)
		VALUES 
			('prize_1', '100 YAK Bonus', 1, true),
			('prize_2', 'Special Badge', 1, true)
		ON CONFLICT (prize_id) DO NOTHING
	`)
	require.NoError(t, err)

	t.Run("Success - valid transaction with correct amount", func(t *testing.T) {
		// Insert a valid balance change
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, validSignature, yakMintAddress, validWallet, "account1", -yakSpinAmount, 1000000000000, 12345, time.Now())
		require.NoError(t, err)

		requestBody := PrizeClaimRequest{
			Signature: validSignature,
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		var resp PrizeClaimResponse
		status, respBody := testPost(t, app, "/v1/prizes/claim", body, map[string]string{
			"Content-Type": "application/json",
		}, &resp)

		assert.Equal(t, 200, status, "Response body: %s", string(respBody))
		assert.Contains(t, []string{"prize_1", "prize_2"}, resp.PrizeID)
		// Prize name should match one of the prizes
		assert.True(t, resp.PrizeName == "100 YAK Bonus" || resp.PrizeName == "Special Badge",
			"Unexpected prize name: %s", resp.PrizeName)
		assert.Equal(t, validWallet, resp.Wallet)

		// Verify it was saved to database
		var dbPrizeID, dbPrizeName, dbWallet string
		err = app.writePool.QueryRow(ctx, `
			SELECT prize_id, prize_name, wallet
			FROM claimed_prizes 
			WHERE signature = $1
		`, validSignature).Scan(&dbPrizeID, &dbPrizeName, &dbWallet)
		assert.NoError(t, err)
		assert.Equal(t, resp.PrizeID, dbPrizeID)
		assert.Equal(t, resp.PrizeName, dbPrizeName)
		assert.Equal(t, validWallet, dbWallet)
	})

	t.Run("Missing required fields - signature", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"wallet": validWallet,
			// Missing signature
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, _ := testPost(t, app, "/v1/prizes/claim", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for missing signature")
	})

	t.Run("Missing required fields - wallet", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"signature": "some_signature",
			// Missing wallet
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, _ := testPost(t, app, "/v1/prizes/claim", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for missing wallet")
	})

	t.Run("Signature already used", func(t *testing.T) {
		// Insert a result for this signature
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO claimed_prizes (wallet, signature, mint, amount, prize_id, prize_name)
			VALUES ($1, $2, $3, $4, $5, $6)
			ON CONFLICT (signature) DO NOTHING
		`, validWallet, "used_signature", yakMintAddress, yakSpinAmount, "prize_1", "100 YAK Bonus")
		require.NoError(t, err)

		// Insert balance change
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "used_signature", yakMintAddress, validWallet, "account2", -yakSpinAmount, 1000000000000, 12346, time.Now())
		require.NoError(t, err)

		requestBody := PrizeClaimRequest{
			Signature: "used_signature",
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/prizes/claim", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for already used signature. Response: %s", string(respBody))
	})

	t.Run("Transaction not found", func(t *testing.T) {
		requestBody := PrizeClaimRequest{
			Signature: "non_existent_signature",
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/prizes/claim", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for non-existent transaction. Response: %s", string(respBody))
	})

	t.Run("Wrong mint - transaction uses different mint", func(t *testing.T) {
		otherMint := "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM"
		// Insert balance change with different mint
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "wrong_mint_sig", otherMint, validWallet, "account3", -yakSpinAmount, 1000000000000, 12347, time.Now())
		require.NoError(t, err)

		requestBody := PrizeClaimRequest{
			Signature: "wrong_mint_sig",
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/prizes/claim", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for wrong mint. Response: %s", string(respBody))
	})

	t.Run("Wrong amount - transaction uses different amount", func(t *testing.T) {
		wrongAmount := int64(100000000000) // 100 YAK instead of 250
		// Insert balance change with wrong amount
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "wrong_amount_sig", yakMintAddress, validWallet, "account_wrong_amount", -wrongAmount, 1000000000000, 12350, time.Now())
		require.NoError(t, err)

		requestBody := PrizeClaimRequest{
			Signature: "wrong_amount_sig",
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/prizes/claim", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for wrong amount. Response: %s", string(respBody))
	})

	t.Run("Wrong wallet - doesn't match transaction owner", func(t *testing.T) {
		// Insert balance change with different owner
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "wrong_wallet_sig", yakMintAddress, otherWallet, "account4", -yakSpinAmount, 1000000000000, 12348, time.Now())
		require.NoError(t, err)

		requestBody := PrizeClaimRequest{
			Signature: "wrong_wallet_sig",
			Wallet:    validWallet, // Different wallet
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/prizes/claim", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for wrong wallet. Response: %s", string(respBody))
	})

	t.Run("Wrong amount - positive change (receiving instead of spending)", func(t *testing.T) {
		// Insert balance change with positive change (receiving, not spending)
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "positive_change_sig", yakMintAddress, validWallet, "account7", yakSpinAmount, 1000000000000, 12351, time.Now())
		require.NoError(t, err)

		requestBody := PrizeClaimRequest{
			Signature: "positive_change_sig",
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/prizes/claim", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for positive change. Response: %s", string(respBody))
	})

	t.Run("Wrong amount - zero change", func(t *testing.T) {
		// Insert balance change with zero change
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "zero_change_sig", yakMintAddress, validWallet, "account8", 0, 1000000000000, 12352, time.Now())
		require.NoError(t, err)

		requestBody := PrizeClaimRequest{
			Signature: "zero_change_sig",
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/prizes/claim", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for zero change. Response: %s", string(respBody))
	})

	t.Run("Multiple valid spins - different signatures", func(t *testing.T) {
		// First spin
		sig1 := "multi_sig_1"
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, sig1, yakMintAddress, validWallet, "account9", -yakSpinAmount, 1000000000000, 12353, time.Now())
		require.NoError(t, err)

		requestBody1 := PrizeClaimRequest{
			Signature: sig1,
			Wallet:    validWallet,
		}

		body1, err := json.Marshal(requestBody1)
		require.NoError(t, err)

		var resp1 PrizeClaimResponse
		status1, _ := testPost(t, app, "/v1/prizes/claim", body1, map[string]string{
			"Content-Type": "application/json",
		}, &resp1)

		assert.Equal(t, 200, status1, "First spin should succeed")
		assert.Equal(t, validWallet, resp1.Wallet)

		// Second spin with different signature
		sig2 := "multi_sig_2"
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, sig2, yakMintAddress, validWallet, "account10", -yakSpinAmount, 1000000000000, 12354, time.Now())
		require.NoError(t, err)

		requestBody2 := PrizeClaimRequest{
			Signature: sig2,
			Wallet:    validWallet,
		}

		body2, err := json.Marshal(requestBody2)
		require.NoError(t, err)

		var resp2 PrizeClaimResponse
		status2, _ := testPost(t, app, "/v1/prizes/claim", body2, map[string]string{
			"Content-Type": "application/json",
		}, &resp2)

		assert.Equal(t, 200, status2, "Second spin should succeed")
		assert.Equal(t, validWallet, resp2.Wallet)

		// Verify both results are in database
		var count int
		err = app.writePool.QueryRow(ctx, `
			SELECT COUNT(*) 
			FROM claimed_prizes 
			WHERE wallet = $1 AND (signature = $2 OR signature = $3)
		`, validWallet, sig1, sig2).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 2, count, "Should have 2 results in database")

		// Verify mint and amount are stored correctly
		var dbMint1, dbMint2 string
		var dbAmount1, dbAmount2 int64
		err = app.writePool.QueryRow(ctx, `
			SELECT mint, amount FROM claimed_prizes WHERE signature = $1
		`, sig1).Scan(&dbMint1, &dbAmount1)
		assert.NoError(t, err)
		assert.Equal(t, yakMintAddress, dbMint1)
		assert.Equal(t, int64(yakSpinAmount), dbAmount1)

		err = app.writePool.QueryRow(ctx, `
			SELECT mint, amount FROM claimed_prizes WHERE signature = $1
		`, sig2).Scan(&dbMint2, &dbAmount2)
		assert.NoError(t, err)
		assert.Equal(t, yakMintAddress, dbMint2)
		assert.Equal(t, int64(yakSpinAmount), dbAmount2)
	})

	t.Run("Invalid JSON body", func(t *testing.T) {
		status, _ := testPost(t, app, "/v1/prizes/claim", []byte("invalid json"), map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for invalid JSON")
	})

	t.Run("Empty body", func(t *testing.T) {
		status, _ := testPost(t, app, "/v1/prizes/claim", []byte("{}"), map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for empty body")
	})
}
