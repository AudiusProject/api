package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http/httptest"
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
			prize_type VARCHAR,
			action_data JSONB,
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
		yakMintAddress       = "ZDaUDL4XFdEct7UgeztrFQAptsvh4ZdhyZDZ1RpxYAK"
		yakClaimAmount       = 2000000000 // 2 YAK with 9 decimals - amount required to claim a prize
		prizeReceiverAddress = "EHd892m3xNWGBuAXnafavqcFjXXUZp9bGecdSDNP2SLR"
		validWallet          = "HLnpSz9h2S4hiLQ43rnSD9XkcUThA7B8hQMKmDaiTLcC"
		validSignature       = "valid_signature_123"
		otherWallet          = "DDT15s6MMNxE4jkyGN46wNYqrgLWofT6WAvWtjYYrCUq"
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
		// Insert a valid balance change (user spending)
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, validSignature, yakMintAddress, validWallet, "account1", -yakClaimAmount, 1000000000000, 12345, time.Now())
		require.NoError(t, err)

		// Insert corresponding positive balance change for prize receiver
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, validSignature, yakMintAddress, prizeReceiverAddress, "receiver_account1", yakClaimAmount, 1000000000000, 12345, time.Now())
		require.NoError(t, err)
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
			INSERT INTO claimed_prizes (wallet, signature, mint, amount, prize_id, prize_name, prize_type, action_data)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (signature) DO NOTHING
		`, validWallet, "used_signature", yakMintAddress, yakClaimAmount, "prize_1", "100 YAK Bonus", nil, nil)
		require.NoError(t, err)

		// Insert balance change (user spending)
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "used_signature", yakMintAddress, validWallet, "account2", -yakClaimAmount, 1000000000000, 12346, time.Now())
		require.NoError(t, err)

		// Insert receiver balance change
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "used_signature", yakMintAddress, prizeReceiverAddress, "receiver_account2", yakClaimAmount, 1000000000000, 12346, time.Now())
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
		`, "wrong_mint_sig", otherMint, validWallet, "account3", -yakClaimAmount, 1000000000000, 12347, time.Now())
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
		wrongAmount := int64(100000000000) // 100 YAK instead of 2
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
		`, "wrong_wallet_sig", yakMintAddress, otherWallet, "account4", -yakClaimAmount, 1000000000000, 12348, time.Now())
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

	t.Run("Success - account matches wallet in addition to owner", func(t *testing.T) {
		accountMatchingSignature := "account_match_sig"
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, accountMatchingSignature, yakMintAddress, validWallet, validWallet, -yakClaimAmount, 1000000000000, 12349, time.Now())
		require.NoError(t, err)

		// Insert corresponding positive balance change for prize receiver
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, accountMatchingSignature, yakMintAddress, prizeReceiverAddress, "receiver_account_match", yakClaimAmount, 1000000000000, 12349, time.Now())
		require.NoError(t, err)

		requestBody := PrizeClaimRequest{
			Signature: accountMatchingSignature,
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		var resp PrizeClaimResponse
		status, respBody := testPost(t, app, "/v1/prizes/claim", body, map[string]string{
			"Content-Type": "application/json",
		}, &resp)

		assert.Equal(t, 200, status, "Should succeed when account matches wallet. Response: %s", string(respBody))
		assert.Contains(t, []string{"prize_1", "prize_2"}, resp.PrizeID)
		assert.Equal(t, validWallet, resp.Wallet)

		// Verify it was saved to database
		var dbPrizeID, dbPrizeName, dbWallet string
		err = app.writePool.QueryRow(ctx, `
			SELECT prize_id, prize_name, wallet
			FROM claimed_prizes 
			WHERE signature = $1
		`, accountMatchingSignature).Scan(&dbPrizeID, &dbPrizeName, &dbWallet)
		assert.NoError(t, err)
		assert.Equal(t, resp.PrizeID, dbPrizeID)
		assert.Equal(t, resp.PrizeName, dbPrizeName)
		assert.Equal(t, validWallet, dbWallet)
	})

	t.Run("Wrong amount - positive change (receiving instead of spending)", func(t *testing.T) {
		// Insert balance change with positive change (receiving, not spending)
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "positive_change_sig", yakMintAddress, validWallet, "account7", yakClaimAmount, 1000000000000, 12351, time.Now())
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
		`, sig1, yakMintAddress, validWallet, "account9", -yakClaimAmount, 1000000000000, 12353, time.Now())
		require.NoError(t, err)

		// Insert receiver balance change for sig1
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, sig1, yakMintAddress, prizeReceiverAddress, "receiver_account9", yakClaimAmount, 1000000000000, 12353, time.Now())
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
		`, sig2, yakMintAddress, validWallet, "account10", -yakClaimAmount, 1000000000000, 12354, time.Now())
		require.NoError(t, err)

		// Insert receiver balance change for sig2
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, sig2, yakMintAddress, prizeReceiverAddress, "receiver_account10", yakClaimAmount, 1000000000000, 12354, time.Now())
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
		assert.Equal(t, int64(yakClaimAmount), dbAmount1)

		err = app.writePool.QueryRow(ctx, `
			SELECT mint, amount FROM claimed_prizes WHERE signature = $1
		`, sig2).Scan(&dbMint2, &dbAmount2)
		assert.NoError(t, err)
		assert.Equal(t, yakMintAddress, dbMint2)
		assert.Equal(t, int64(yakClaimAmount), dbAmount2)
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

	// Helper function to test coin airdrop prizes with different amounts
	testCoinAirdropPrize := func(t *testing.T, amount int64, prizeID, prizeName, signature string) {
		// Create reward_codes table if it doesn't exist
		_, err := app.writePool.Exec(ctx, `
			CREATE TABLE IF NOT EXISTS reward_codes (
				code VARCHAR NOT NULL PRIMARY KEY,
				mint VARCHAR NOT NULL,
				reward_address VARCHAR,
				amount BIGINT NOT NULL,
				remaining_uses INT NOT NULL DEFAULT 1,
				signature VARCHAR,
				created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
			)
		`)
		require.NoError(t, err)

		// Deactivate other prizes and insert only the airdrop prize to ensure it's selected
		_, err = app.writePool.Exec(ctx, `UPDATE prizes SET is_active = false`)
		require.NoError(t, err)

		// Insert a prize with coin_airdrop metadata
		airdropMetadata := fmt.Sprintf(`{"type": "coin_airdrop", "amount": %d}`, amount)
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO prizes (prize_id, name, weight, is_active, metadata)
			VALUES ($1, $2, 1, true, $3::jsonb)
			ON CONFLICT (prize_id) DO UPDATE SET metadata = $3::jsonb, is_active = true
		`, prizeID, prizeName, airdropMetadata)
		require.NoError(t, err)

		// Insert a valid balance change (user spending)
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, signature, yakMintAddress, validWallet, fmt.Sprintf("account_airdrop_%s", signature), -yakClaimAmount, 1000000000000, 12360, time.Now())
		require.NoError(t, err)

		// Insert receiver balance change
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, signature, yakMintAddress, prizeReceiverAddress, fmt.Sprintf("receiver_account_airdrop_%s", signature), yakClaimAmount, 1000000000000, 12360, time.Now())
		require.NoError(t, err)

		requestBody := PrizeClaimRequest{
			Signature: signature,
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		var resp PrizeClaimResponse
		status, respBody := testPost(t, app, "/v1/prizes/claim", body, map[string]string{
			"Content-Type": "application/json",
		}, &resp)

		assert.Equal(t, 200, status, "Response body: %s", string(respBody))
		assert.Equal(t, prizeID, resp.PrizeID)
		assert.Equal(t, prizeName, resp.PrizeName)
		assert.Equal(t, validWallet, resp.Wallet)
		assert.NotNil(t, resp.PrizeType, "Prize type should be set")
		assert.Equal(t, "coin_airdrop", *resp.PrizeType)
		assert.NotNil(t, resp.ActionData, "Action data should be set")

		// Parse action_data to verify it contains code and url
		var actionData map[string]string
		err = json.Unmarshal(resp.ActionData, &actionData)
		assert.NoError(t, err, "Action data should be valid JSON")
		assert.Contains(t, actionData, "code", "Action data should contain code")
		assert.Contains(t, actionData, "url", "Action data should contain url")
		assert.NotEmpty(t, actionData["code"], "Code should not be empty")
		assert.NotEmpty(t, actionData["url"], "URL should not be empty")
		assert.Contains(t, actionData["url"], "/coins/YAK/redeem/", "URL should contain redeem path")
		assert.Contains(t, actionData["url"], actionData["code"], "URL should contain the code")

		// Verify it was saved to database with correct prize_type and action_data
		var dbPrizeID, dbPrizeName, dbWallet, dbPrizeType string
		var dbActionData json.RawMessage
		err = app.writePool.QueryRow(ctx, `
			SELECT prize_id, prize_name, wallet, prize_type, action_data
			FROM claimed_prizes 
			WHERE signature = $1
		`, signature).Scan(&dbPrizeID, &dbPrizeName, &dbWallet, &dbPrizeType, &dbActionData)
		assert.NoError(t, err)
		assert.Equal(t, prizeID, dbPrizeID)
		assert.Equal(t, prizeName, dbPrizeName)
		assert.Equal(t, validWallet, dbWallet)
		assert.Equal(t, "coin_airdrop", dbPrizeType)

		// Verify action_data in database matches response
		var dbActionDataMap map[string]string
		err = json.Unmarshal(dbActionData, &dbActionDataMap)
		assert.NoError(t, err)
		assert.Equal(t, actionData["code"], dbActionDataMap["code"])
		assert.Equal(t, actionData["url"], dbActionDataMap["url"])

		// Verify reward code was created in reward_codes table with correct amount
		var dbCode, dbMint string
		var dbAmount int64
		var dbRemainingUses int
		err = app.writePool.QueryRow(ctx, `
			SELECT code, mint, amount, remaining_uses
			FROM reward_codes
			WHERE code = $1
		`, actionData["code"]).Scan(&dbCode, &dbMint, &dbAmount, &dbRemainingUses)
		assert.NoError(t, err, "Reward code should exist in database")
		assert.Equal(t, actionData["code"], dbCode)
		assert.Equal(t, yakMintAddress, dbMint)
		assert.Equal(t, amount, dbAmount, "Reward code amount should match prize amount")
		assert.Equal(t, 1, dbRemainingUses, "Remaining uses should be 1")
	}

	t.Run("Success - 1 YAK coin airdrop prize with redeem code", func(t *testing.T) {
		testCoinAirdropPrize(t, 1, "prize_1_yak", "1 YAK Airdrop", "airdrop_sig_1_yak")
	})

	t.Run("Success - 2 YAK coin airdrop prize with redeem code", func(t *testing.T) {
		testCoinAirdropPrize(t, 2, "prize_2_yak", "2 YAK Airdrop", "airdrop_sig_2_yak")
	})

	t.Run("Success - 3 YAK coin airdrop prize with redeem code", func(t *testing.T) {
		testCoinAirdropPrize(t, 3, "prize_3_yak", "3 YAK Airdrop", "airdrop_sig_3_yak")
	})

	t.Run("Success - direct download prize", func(t *testing.T) {
		// Insert a prize with download metadata
		downloadPrizeID := "prize_download"
		downloadURL := "https://example.com/downloads/exclusive-track.mp3"
		downloadMetadata := fmt.Sprintf(`{"type": "download", "download_url": "%s"}`, downloadURL)
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO prizes (prize_id, name, weight, is_active, metadata)
			VALUES ($1, 'Exclusive Track Download', 1, true, $2::jsonb)
			ON CONFLICT (prize_id) DO UPDATE SET metadata = $2::jsonb, is_active = true
		`, downloadPrizeID, downloadMetadata)
		require.NoError(t, err)

		// Deactivate other prizes to ensure this one is selected
		_, err = app.writePool.Exec(ctx, `UPDATE prizes SET is_active = false WHERE prize_id != $1`, downloadPrizeID)
		require.NoError(t, err)

		// Insert a valid balance change (user spending)
		downloadSignature := "download_sig_123"
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, downloadSignature, yakMintAddress, validWallet, "account_download", -yakClaimAmount, 1000000000000, 12370, time.Now())
		require.NoError(t, err)

		// Insert receiver balance change
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, downloadSignature, yakMintAddress, prizeReceiverAddress, "receiver_account_download", yakClaimAmount, 1000000000000, 12370, time.Now())
		require.NoError(t, err)

		requestBody := PrizeClaimRequest{
			Signature: downloadSignature,
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		var resp PrizeClaimResponse
		status, respBody := testPost(t, app, "/v1/prizes/claim", body, map[string]string{
			"Content-Type": "application/json",
		}, &resp)

		assert.Equal(t, 200, status, "Response body: %s", string(respBody))
		assert.Equal(t, downloadPrizeID, resp.PrizeID)
		assert.Equal(t, "Exclusive Track Download", resp.PrizeName)
		assert.Equal(t, validWallet, resp.Wallet)
		assert.NotNil(t, resp.PrizeType, "Prize type should be set")
		assert.Equal(t, "download", *resp.PrizeType)
		assert.NotNil(t, resp.ActionData, "Action data should be set")

		// Parse action_data to verify it contains download_url
		var actionData map[string]string
		err = json.Unmarshal(resp.ActionData, &actionData)
		assert.NoError(t, err, "Action data should be valid JSON")
		assert.Contains(t, actionData, "download_url", "Action data should contain download_url")
		assert.Equal(t, downloadURL, actionData["download_url"], "Download URL should match")

		// Verify it was saved to database with correct prize_type and action_data
		var dbPrizeID, dbPrizeName, dbWallet, dbPrizeType string
		var dbActionData json.RawMessage
		err = app.writePool.QueryRow(ctx, `
			SELECT prize_id, prize_name, wallet, prize_type, action_data
			FROM claimed_prizes 
			WHERE signature = $1
		`, downloadSignature).Scan(&dbPrizeID, &dbPrizeName, &dbWallet, &dbPrizeType, &dbActionData)
		assert.NoError(t, err)
		assert.Equal(t, downloadPrizeID, dbPrizeID)
		assert.Equal(t, "Exclusive Track Download", dbPrizeName)
		assert.Equal(t, validWallet, dbWallet)
		assert.Equal(t, "download", dbPrizeType)

		// Verify action_data in database matches response
		var dbActionDataMap map[string]string
		err = json.Unmarshal(dbActionData, &dbActionDataMap)
		assert.NoError(t, err)
		assert.Equal(t, downloadURL, dbActionDataMap["download_url"])
	})

	t.Run("GET /v1/prizes - URLs not leaked in public endpoint", func(t *testing.T) {
		// Insert prizes with sensitive URLs
		downloadURL := "https://example.com/downloads/exclusive-track.mp3"
		typeformURL := "https://typeform.com/to/abc123"
		downloadMetadata := fmt.Sprintf(`{"type": "download", "download_url": "%s"}`, downloadURL)
		typeformMetadata := fmt.Sprintf(`{"type": "typeform", "url": "%s"}`, typeformURL)
		coinMetadata := fmt.Sprintf(`{"type": "coin_airdrop", "amount": %d}`, 1)

		_, err := app.writePool.Exec(ctx, `
			INSERT INTO prizes (prize_id, name, weight, is_active, metadata)
			VALUES 
				('prize_download_test', 'Exclusive Download', 1, true, $1::jsonb),
				('prize_typeform_test', 'Typeform Survey', 1, true, $2::jsonb),
				('prize_coin_test', '1 YAK', 1, true, $3::jsonb)
			ON CONFLICT (prize_id) DO UPDATE SET metadata = EXCLUDED.metadata, is_active = true
		`, downloadMetadata, typeformMetadata, coinMetadata)
		require.NoError(t, err)

		// Test the public GET endpoint
		var response struct {
			Data []PrizePublicResponse `json:"data"`
		}
		status, body := testGet(t, app, "/v1/prizes", &response)

		assert.Equal(t, 200, status, "Response body: %s", string(body))
		assert.GreaterOrEqual(t, len(response.Data), 3, "Should return at least 3 prizes")

		// Find the download prize
		var downloadPrize *PrizePublicResponse
		for i := range response.Data {
			if response.Data[i].PrizeID == "prize_download_test" {
				downloadPrize = &response.Data[i]
				break
			}
		}
		require.NotNil(t, downloadPrize, "Download prize should be in response")
		assert.Equal(t, "Exclusive Download", downloadPrize.Name)
		assert.Equal(t, "download", downloadPrize.PublicMeta["type"])
		// Verify download_url is NOT in the response
		assert.NotContains(t, string(body), downloadURL, "Download URL should not be leaked")
		if downloadPrize.PublicMeta != nil {
			_, hasDownloadURL := downloadPrize.PublicMeta["download_url"]
			assert.False(t, hasDownloadURL, "download_url should not be in public metadata")
		}

		// Find the typeform prize
		var typeformPrize *PrizePublicResponse
		for i := range response.Data {
			if response.Data[i].PrizeID == "prize_typeform_test" {
				typeformPrize = &response.Data[i]
				break
			}
		}
		require.NotNil(t, typeformPrize, "Typeform prize should be in response")
		assert.Equal(t, "Typeform Survey", typeformPrize.Name)
		assert.Equal(t, "typeform", typeformPrize.PublicMeta["type"])
		// Verify typeform URL is NOT in the response
		assert.NotContains(t, string(body), typeformURL, "Typeform URL should not be leaked")
		if typeformPrize.PublicMeta != nil {
			_, hasURL := typeformPrize.PublicMeta["url"]
			assert.False(t, hasURL, "url should not be in public metadata")
		}

		// Find the coin prize
		var coinPrize *PrizePublicResponse
		for i := range response.Data {
			if response.Data[i].PrizeID == "prize_coin_test" {
				coinPrize = &response.Data[i]
				break
			}
		}
		require.NotNil(t, coinPrize, "Coin prize should be in response")
		assert.Equal(t, "1 YAK", coinPrize.Name)
		assert.Equal(t, "coin_airdrop", coinPrize.PublicMeta["type"])
		// Amount can be shown (it's just a number), but no redeem code/URL
		if coinPrize.PublicMeta != nil {
			assert.Equal(t, float64(1), coinPrize.PublicMeta["amount"])
			_, hasCode := coinPrize.PublicMeta["code"]
			assert.False(t, hasCode, "code should not be in public metadata")
			_, hasURL := coinPrize.PublicMeta["url"]
			assert.False(t, hasURL, "url should not be in public metadata")
		}
	})

	t.Run("GET /v1/prizes - URLs not leaked in public endpoint", func(t *testing.T) {
		// Insert prizes with sensitive URLs
		downloadURL := "https://example.com/downloads/exclusive-track.mp3"
		typeformURL := "https://typeform.com/to/abc123"
		downloadMetadata := fmt.Sprintf(`{"type": "download", "download_url": "%s"}`, downloadURL)
		typeformMetadata := fmt.Sprintf(`{"type": "typeform", "url": "%s"}`, typeformURL)
		coinMetadata := fmt.Sprintf(`{"type": "coin_airdrop", "amount": %d}`, 1)

		_, err := app.writePool.Exec(ctx, `
			INSERT INTO prizes (prize_id, name, weight, is_active, metadata)
			VALUES 
				('prize_download_test', 'Exclusive Download', 1, true, $1::jsonb),
				('prize_typeform_test', 'Typeform Survey', 1, true, $2::jsonb),
				('prize_coin_test', '1 YAK', 1, true, $3::jsonb)
			ON CONFLICT (prize_id) DO UPDATE SET metadata = EXCLUDED.metadata, is_active = true
		`, downloadMetadata, typeformMetadata, coinMetadata)
		require.NoError(t, err)

		// Test the public GET endpoint
		req := httptest.NewRequest("GET", "/v1/prizes", nil)
		res, err := app.Test(req, -1)
		require.NoError(t, err)
		body, _ := io.ReadAll(res.Body)

		assert.Equal(t, 200, res.StatusCode, "Response body: %s", string(body))

		var response struct {
			Data []PrizePublicResponse `json:"data"`
		}
		err = json.Unmarshal(body, &response)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(response.Data), 3, "Should return at least 3 prizes")

		// Verify URLs are NOT in the response body
		assert.NotContains(t, string(body), downloadURL, "Download URL should not be leaked")
		assert.NotContains(t, string(body), typeformURL, "Typeform URL should not be leaked")

		// Find the download prize
		var downloadPrize *PrizePublicResponse
		for i := range response.Data {
			if response.Data[i].PrizeID == "prize_download_test" {
				downloadPrize = &response.Data[i]
				break
			}
		}
		require.NotNil(t, downloadPrize, "Download prize should be in response")
		assert.Equal(t, "Exclusive Download", downloadPrize.Name)
		if downloadPrize.PublicMeta != nil {
			assert.Equal(t, "download", downloadPrize.PublicMeta["type"])
			// Verify download_url is NOT in the public metadata
			_, hasDownloadURL := downloadPrize.PublicMeta["download_url"]
			assert.False(t, hasDownloadURL, "download_url should not be in public metadata")
		}

		// Find the typeform prize
		var typeformPrize *PrizePublicResponse
		for i := range response.Data {
			if response.Data[i].PrizeID == "prize_typeform_test" {
				typeformPrize = &response.Data[i]
				break
			}
		}
		require.NotNil(t, typeformPrize, "Typeform prize should be in response")
		assert.Equal(t, "Typeform Survey", typeformPrize.Name)
		if typeformPrize.PublicMeta != nil {
			assert.Equal(t, "typeform", typeformPrize.PublicMeta["type"])
			// Verify url is NOT in the public metadata
			_, hasURL := typeformPrize.PublicMeta["url"]
			assert.False(t, hasURL, "url should not be in public metadata")
		}

		// Find the coin prize
		var coinPrize *PrizePublicResponse
		for i := range response.Data {
			if response.Data[i].PrizeID == "prize_coin_test" {
				coinPrize = &response.Data[i]
				break
			}
		}
		require.NotNil(t, coinPrize, "Coin prize should be in response")
		assert.Equal(t, "1 YAK", coinPrize.Name)
		if coinPrize.PublicMeta != nil {
			assert.Equal(t, "coin_airdrop", coinPrize.PublicMeta["type"])
			// Amount can be shown (it's just a number), but no redeem code/URL
			assert.Equal(t, float64(1), coinPrize.PublicMeta["amount"])
			_, hasCode := coinPrize.PublicMeta["code"]
			assert.False(t, hasCode, "code should not be in public metadata")
			_, hasURL := coinPrize.PublicMeta["url"]
			assert.False(t, hasURL, "url should not be in public metadata")
		}
	})

	t.Run("GET /v1/wallet/:wallet/prizes - Success - returns claimed prizes (public, no auth required)", func(t *testing.T) {
		wallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"
		signature1 := "test_signature_wallet_prizes_1"
		signature2 := "test_signature_wallet_prizes_2"
		otherWallet := "0xc3d1d41e6872ffbd15c473d14fc3a9250be5b5e0"
		otherSignature := "test_signature_other_wallet"

		// Insert prizes
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO prizes (prize_id, name, weight, is_active)
			VALUES 
				('prize_wallet_test_1', 'Test Prize 1', 1, true),
				('prize_wallet_test_2', 'Test Prize 2', 1, true)
			ON CONFLICT (prize_id) DO UPDATE SET is_active = true
		`)
		require.NoError(t, err)

		// Insert claimed prizes for the test wallet with sensitive action_data
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO claimed_prizes (wallet, signature, mint, amount, prize_id, prize_name, prize_type, action_data)
			VALUES 
				($1, $2, $3, $4, 'prize_wallet_test_1', 'Test Prize 1', 'download', '{"download_url": "https://example.com/file1.zip"}'::jsonb),
				($1, $5, $3, $4, 'prize_wallet_test_2', 'Test Prize 2', 'coin_airdrop', '{"code": "ABC123", "url": "https://example.com/redeem"}'::jsonb)
		`, wallet, signature1, yakMintAddress, yakClaimAmount, signature2)
		require.NoError(t, err)

		// Insert a claimed prize for a different wallet (should not appear)
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO claimed_prizes (wallet, signature, mint, amount, prize_id, prize_name)
			VALUES ($1, $2, $3, $4, 'prize_wallet_test_1', 'Test Prize 1')
		`, otherWallet, otherSignature, yakMintAddress, yakClaimAmount)
		require.NoError(t, err)

		// Test public request (no authentication required)
		var response struct {
			Data []ClaimedPrizeResponse `json:"data"`
		}
		status, body := testGet(t, app, fmt.Sprintf("/v1/wallet/%s/prizes", wallet), &response)

		assert.Equal(t, 200, status, "Response body: %s", string(body))
		assert.Equal(t, 2, len(response.Data), "Should return 2 claimed prizes for the wallet")

		// Verify both prizes are returned but WITHOUT sensitive action_data
		prize1Found := false
		prize2Found := false
		for _, prize := range response.Data {
			assert.Equal(t, wallet, prize.Wallet, "All prizes should belong to the requested wallet")
			// Verify action_data is NOT included (security - sensitive URLs/codes excluded)
			assert.Nil(t, prize.ActionData, "action_data should not be returned in public endpoint")

			if prize.Signature == signature1 {
				prize1Found = true
				assert.Equal(t, "prize_wallet_test_1", prize.PrizeID)
				assert.Equal(t, "Test Prize 1", prize.PrizeName)
				assert.NotNil(t, prize.PrizeType)
				assert.Equal(t, "download", *prize.PrizeType)
			}
			if prize.Signature == signature2 {
				prize2Found = true
				assert.Equal(t, "prize_wallet_test_2", prize.PrizeID)
				assert.Equal(t, "Test Prize 2", prize.PrizeName)
				assert.NotNil(t, prize.PrizeType)
				assert.Equal(t, "coin_airdrop", *prize.PrizeType)
			}
			// Verify other wallet's prize is not included
			assert.NotEqual(t, otherSignature, prize.Signature)
		}
		assert.True(t, prize1Found, "Prize 1 should be in response")
		assert.True(t, prize2Found, "Prize 2 should be in response")

		// Verify sensitive data is not in response body
		assert.NotContains(t, string(body), "file1.zip", "Download URL should not be leaked")
		assert.NotContains(t, string(body), "ABC123", "Redeem code should not be leaked")
		assert.NotContains(t, string(body), "https://example.com", "URLs should not be leaked")
	})

	t.Run("GET /v1/wallet/:wallet/prizes - Empty result for wallet with no prizes", func(t *testing.T) {
		wallet := "0x7d273271690538cf855e5b3002a0dd8c154bb060"

		// Ensure no prizes exist for this wallet (clean up if any)
		_, err := app.writePool.Exec(ctx, `DELETE FROM claimed_prizes WHERE wallet = $1`, wallet)
		require.NoError(t, err)

		var response struct {
			Data []ClaimedPrizeResponse `json:"data"`
		}
		// Public endpoint - no authentication required
		status, body := testGet(t, app, fmt.Sprintf("/v1/wallet/%s/prizes", wallet), &response)

		assert.Equal(t, 200, status, "Response body: %s", string(body))
		assert.Equal(t, 0, len(response.Data), "Should return empty array for wallet with no prizes")
	})
}
