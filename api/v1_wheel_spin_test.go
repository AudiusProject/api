package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestV1WheelSpin(t *testing.T) {
	app := emptyTestApp(t)

	// Run migration to create wheel_spin_results table
	ctx := context.Background()
	// Drop table if it exists to ensure clean state
	_, err := app.writePool.Exec(ctx, `DROP TABLE IF EXISTS wheel_spin_results CASCADE`)
	require.NoError(t, err)

	_, err = app.writePool.Exec(ctx, `
		CREATE TABLE wheel_spin_results (
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
		CREATE INDEX wheel_spin_results_wallet_idx ON wheel_spin_results (wallet);
		CREATE INDEX wheel_spin_results_signature_idx ON wheel_spin_results (signature);
		CREATE INDEX wheel_spin_results_mint_idx ON wheel_spin_results (mint);
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

	// Create wheel_spin_configs table
	_, err = app.writePool.Exec(ctx, `DROP TABLE IF EXISTS wheel_spin_configs CASCADE`)
	require.NoError(t, err)

	_, err = app.writePool.Exec(ctx, `
		CREATE TABLE wheel_spin_configs (
			id SERIAL PRIMARY KEY,
			mint VARCHAR NOT NULL,
			amount BIGINT NOT NULL,
			name VARCHAR,
			description TEXT,
			is_active BOOLEAN NOT NULL DEFAULT true,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(mint, amount)
		);
		CREATE INDEX wheel_spin_configs_mint_idx ON wheel_spin_configs (mint);
		CREATE INDEX wheel_spin_configs_active_idx ON wheel_spin_configs (is_active);
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
		yakMintAddress  = "ZDaUDL4XFdEct7UgeztrFQAptsvh4ZdhyZDZ1RpxYAK"
		yakSpinAmount   = 250000000000 // 250 YAK with 9 decimals
		validWallet     = "HLnpSz9h2S4hiLQ43rnSD9XkcUThA7B8hQMKmDaiTLcC"
		validSignature  = "valid_signature_123"
		otherWallet     = "DDT15s6MMNxE4jkyGN46wNYqrgLWofT6WAvWtjYYrCUq"
		otherMint       = "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM"
		audioMint       = "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM"
		audioSpinAmount = 25000000000 // 250 AUDIO with 8 decimals
	)

	// Insert test coin data
	_, err = app.writePool.Exec(ctx, `
		INSERT INTO artist_coins (mint, ticker, user_id, decimals)
		VALUES 
			($1, 'YAK', 1, 9),
			($2, 'AUDIO', 2, 8)
		ON CONFLICT (mint) DO NOTHING
	`, yakMintAddress, audioMint)
	require.NoError(t, err)

	// Create wheel_spin_prizes table
	_, err = app.writePool.Exec(ctx, `DROP TABLE IF EXISTS wheel_spin_prizes CASCADE`)
	require.NoError(t, err)

	_, err = app.writePool.Exec(ctx, `
		CREATE TABLE wheel_spin_prizes (
			id SERIAL PRIMARY KEY,
			config_id INT NOT NULL,
			prize_id VARCHAR NOT NULL,
			name VARCHAR NOT NULL,
			description TEXT,
			weight INT NOT NULL DEFAULT 1,
			is_active BOOLEAN NOT NULL DEFAULT true,
			metadata JSONB,
			created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
			FOREIGN KEY (config_id) REFERENCES wheel_spin_configs(id) ON DELETE CASCADE,
			UNIQUE(config_id, prize_id)
		);
		CREATE INDEX wheel_spin_prizes_config_id_idx ON wheel_spin_prizes (config_id);
		CREATE INDEX wheel_spin_prizes_active_idx ON wheel_spin_prizes (config_id, is_active);
	`)
	require.NoError(t, err)

	// Insert test wheel spin configs and get their IDs
	var yakConfigID, audioConfigID int
	err = app.writePool.QueryRow(ctx, `
		INSERT INTO wheel_spin_configs (mint, amount, name, is_active)
		VALUES ($1, $2, 'YAK Spin', true)
		ON CONFLICT (mint, amount) DO UPDATE SET is_active = true
		RETURNING id
	`, yakMintAddress, yakSpinAmount).Scan(&yakConfigID)
	require.NoError(t, err)

	err = app.writePool.QueryRow(ctx, `
		INSERT INTO wheel_spin_configs (mint, amount, name, is_active)
		VALUES ($1, $2, 'AUDIO Spin', true)
		ON CONFLICT (mint, amount) DO UPDATE SET is_active = true
		RETURNING id
	`, audioMint, audioSpinAmount).Scan(&audioConfigID)
	require.NoError(t, err)

	// Insert test prizes for YAK config
	_, err = app.writePool.Exec(ctx, `
		INSERT INTO wheel_spin_prizes (config_id, prize_id, name, weight, is_active)
		VALUES 
			($1, 'prize_1', '100 YAK Bonus', 1, true),
			($1, 'prize_2', 'Special Badge', 1, true)
		ON CONFLICT (config_id, prize_id) DO NOTHING
	`, yakConfigID)
	require.NoError(t, err)

	// Insert test prizes for AUDIO config
	_, err = app.writePool.Exec(ctx, `
		INSERT INTO wheel_spin_prizes (config_id, prize_id, name, weight, is_active)
		VALUES 
			($1, 'prize_1', '100 AUDIO Bonus', 1, true),
			($1, 'prize_2', 'Special Badge', 1, true)
		ON CONFLICT (config_id, prize_id) DO NOTHING
	`, audioConfigID)
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

		requestBody := WheelSpinRequest{
			Signature: validSignature,
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		var resp WheelSpinResponse
		status, respBody := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
			"Content-Type": "application/json",
		}, &resp)

		assert.Equal(t, 200, status, "Response body: %s", string(respBody))
		assert.Contains(t, []string{"prize_1", "prize_2"}, resp.PrizeID)
		// Prize name should match one of the YAK config prizes
		assert.True(t, resp.PrizeName == "100 YAK Bonus" || resp.PrizeName == "Special Badge",
			"Unexpected prize name: %s", resp.PrizeName)
		assert.Equal(t, validWallet, resp.Wallet)
		assert.Equal(t, yakMintAddress, resp.Mint)
		assert.Equal(t, int64(yakSpinAmount), resp.Amount)

		// Verify it was saved to database
		var dbPrizeID, dbPrizeName, dbWallet, dbMint string
		var dbAmount int64
		err = app.writePool.QueryRow(ctx, `
			SELECT prize_id, prize_name, wallet, mint, amount
			FROM wheel_spin_results 
			WHERE signature = $1
		`, validSignature).Scan(&dbPrizeID, &dbPrizeName, &dbWallet, &dbMint, &dbAmount)
		assert.NoError(t, err)
		assert.Equal(t, resp.PrizeID, dbPrizeID)
		assert.Equal(t, resp.PrizeName, dbPrizeName)
		assert.Equal(t, validWallet, dbWallet)
		assert.Equal(t, yakMintAddress, dbMint)
		assert.Equal(t, int64(yakSpinAmount), dbAmount)
	})

	t.Run("Missing required fields - signature", func(t *testing.T) {
		requestBody := map[string]interface{}{
			"wallet": validWallet,
			// Missing signature
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, _ := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
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

		status, _ := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for missing wallet")
	})

	t.Run("Signature already used", func(t *testing.T) {
		// Insert a result for this signature
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO wheel_spin_results (wallet, signature, mint, amount, prize_id, prize_name)
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

		requestBody := WheelSpinRequest{
			Signature: "used_signature",
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for already used signature. Response: %s", string(respBody))
	})

	t.Run("Transaction not found", func(t *testing.T) {
		requestBody := WheelSpinRequest{
			Signature: "non_existent_signature",
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for non-existent transaction. Response: %s", string(respBody))
	})

	t.Run("Wrong mint - provided mint doesn't match transaction", func(t *testing.T) {
		// Insert balance change with one mint
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "wrong_mint_sig", yakMintAddress, validWallet, "account3", -yakSpinAmount, 1000000000000, 12347, time.Now())
		require.NoError(t, err)

		requestBody := WheelSpinRequest{
			Signature: "wrong_mint_sig",
			Wallet:    validWallet,
			Mint:      otherMint, // Different mint than transaction
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for wrong mint. Response: %s", string(respBody))
	})

	t.Run("Success - different coin (AUDIO with 8 decimals)", func(t *testing.T) {
		sig := "audio_sig_123"
		// Insert balance change for AUDIO (8 decimals)
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, sig, audioMint, validWallet, "account_audio", -audioSpinAmount, 1000000000000, 12348, time.Now())
		require.NoError(t, err)

		requestBody := WheelSpinRequest{
			Signature: sig,
			Wallet:    validWallet,
			Mint:      audioMint,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		var resp WheelSpinResponse
		status, respBody := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
			"Content-Type": "application/json",
		}, &resp)

		assert.Equal(t, 200, status, "Response body: %s", string(respBody))
		assert.Equal(t, audioMint, resp.Mint)
		assert.Equal(t, int64(audioSpinAmount), resp.Amount)
	})

	t.Run("Success - mint inferred from transaction", func(t *testing.T) {
		sig := "inferred_mint_sig"
		// Insert balance change without specifying mint in request
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, sig, yakMintAddress, validWallet, "account_inferred", -yakSpinAmount, 1000000000000, 12349, time.Now())
		require.NoError(t, err)

		requestBody := WheelSpinRequest{
			Signature: sig,
			Wallet:    validWallet,
			// Mint not provided - should be inferred
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		var resp WheelSpinResponse
		status, respBody := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
			"Content-Type": "application/json",
		}, &resp)

		assert.Equal(t, 200, status, "Response body: %s", string(respBody))
		assert.Equal(t, yakMintAddress, resp.Mint)
		assert.Equal(t, int64(yakSpinAmount), resp.Amount)
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

		requestBody := WheelSpinRequest{
			Signature: "wrong_wallet_sig",
			Wallet:    validWallet, // Different wallet
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for wrong wallet. Response: %s", string(respBody))
	})

	t.Run("Invalid config - amount not in configs", func(t *testing.T) {
		// Insert balance change with amount that doesn't match any config
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "wrong_amount_sig", yakMintAddress, validWallet, "account5", -100000000000, 1000000000000, 12349, time.Now())
		require.NoError(t, err)

		requestBody := WheelSpinRequest{
			Signature: "wrong_amount_sig",
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for invalid config. Response: %s", string(respBody))
		assert.Contains(t, string(respBody), "not a valid wheel spin configuration")
	})

	t.Run("Invalid config - coin not in configs", func(t *testing.T) {
		// Insert balance change for a coin that doesn't have a config
		unknownMint := "UnknownMint123456789012345678901234567890"
		unknownAmount := int64(250000000000)
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "unknown_coin_sig", unknownMint, validWallet, "account6", -unknownAmount, 1000000000000, 12350, time.Now())
		require.NoError(t, err)

		requestBody := WheelSpinRequest{
			Signature: "unknown_coin_sig",
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for coin not in configs. Response: %s", string(respBody))
		assert.Contains(t, string(respBody), "not a valid wheel spin configuration")
	})

	t.Run("Invalid config - inactive config", func(t *testing.T) {
		// Insert an inactive config
		inactiveMint := "InactiveMint123456789012345678901234567"
		inactiveAmount := int64(250000000000)
		var inactiveConfigID int
		err := app.writePool.QueryRow(ctx, `
			INSERT INTO wheel_spin_configs (mint, amount, name, is_active)
			VALUES ($1, $2, 'Inactive Spin', false)
			ON CONFLICT (mint, amount) DO UPDATE SET is_active = false
			RETURNING id
		`, inactiveMint, inactiveAmount).Scan(&inactiveConfigID)
		require.NoError(t, err)

		// Insert a prize for the inactive config (shouldn't matter since config is inactive)
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO wheel_spin_prizes (config_id, prize_id, name, weight, is_active)
			VALUES ($1, 'prize_1', 'Test Prize', 1, true)
			ON CONFLICT (config_id, prize_id) DO NOTHING
		`, inactiveConfigID)
		require.NoError(t, err)

		// Insert balance change
		_, err = app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "inactive_config_sig", inactiveMint, validWallet, "account7", -inactiveAmount, 1000000000000, 12351, time.Now())
		require.NoError(t, err)

		requestBody := WheelSpinRequest{
			Signature: "inactive_config_sig",
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for inactive config. Response: %s", string(respBody))
		assert.Contains(t, string(respBody), "not a valid wheel spin configuration")
	})

	t.Run("Positive change - should be negative for spend", func(t *testing.T) {
		// Insert balance change with positive change (receiving, not spending)
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "positive_change_sig", yakMintAddress, validWallet, "account7", yakSpinAmount, 1000000000000, 12351, time.Now())
		require.NoError(t, err)

		requestBody := WheelSpinRequest{
			Signature: "positive_change_sig",
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for positive change. Response: %s", string(respBody))
	})

	t.Run("Zero change - should fail", func(t *testing.T) {
		// Insert balance change with zero change
		_, err := app.writePool.Exec(ctx, `
			INSERT INTO sol_token_account_balance_changes 
			(signature, mint, owner, account, change, balance, slot, block_timestamp)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT DO NOTHING
		`, "zero_change_sig", yakMintAddress, validWallet, "account8", 0, 1000000000000, 12352, time.Now())
		require.NoError(t, err)

		requestBody := WheelSpinRequest{
			Signature: "zero_change_sig",
			Wallet:    validWallet,
		}

		body, err := json.Marshal(requestBody)
		require.NoError(t, err)

		status, respBody := testPost(t, app, "/v1/wheel/spin", body, map[string]string{
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

		requestBody1 := WheelSpinRequest{
			Signature: sig1,
			Wallet:    validWallet,
		}

		body1, err := json.Marshal(requestBody1)
		require.NoError(t, err)

		var resp1 WheelSpinResponse
		status1, _ := testPost(t, app, "/v1/wheel/spin", body1, map[string]string{
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

		requestBody2 := WheelSpinRequest{
			Signature: sig2,
			Wallet:    validWallet,
		}

		body2, err := json.Marshal(requestBody2)
		require.NoError(t, err)

		var resp2 WheelSpinResponse
		status2, _ := testPost(t, app, "/v1/wheel/spin", body2, map[string]string{
			"Content-Type": "application/json",
		}, &resp2)

		assert.Equal(t, 200, status2, "Second spin should succeed")
		assert.Equal(t, validWallet, resp2.Wallet)

		// Verify both results are in database
		var count int
		err = app.writePool.QueryRow(ctx, `
			SELECT COUNT(*) 
			FROM wheel_spin_results 
			WHERE wallet = $1 AND (signature = $2 OR signature = $3)
		`, validWallet, sig1, sig2).Scan(&count)
		assert.NoError(t, err)
		assert.Equal(t, 2, count, "Should have 2 results in database")

		// Verify mint and amount are stored correctly
		var dbMint1, dbMint2 string
		var dbAmount1, dbAmount2 int64
		err = app.writePool.QueryRow(ctx, `
			SELECT mint, amount FROM wheel_spin_results WHERE signature = $1
		`, sig1).Scan(&dbMint1, &dbAmount1)
		assert.NoError(t, err)
		assert.Equal(t, yakMintAddress, dbMint1)
		assert.Equal(t, int64(yakSpinAmount), dbAmount1)

		err = app.writePool.QueryRow(ctx, `
			SELECT mint, amount FROM wheel_spin_results WHERE signature = $1
		`, sig2).Scan(&dbMint2, &dbAmount2)
		assert.NoError(t, err)
		assert.Equal(t, yakMintAddress, dbMint2)
		assert.Equal(t, int64(yakSpinAmount), dbAmount2)
	})

	t.Run("Invalid JSON body", func(t *testing.T) {
		status, _ := testPost(t, app, "/v1/wheel/spin", []byte("invalid json"), map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for invalid JSON")
	})

	t.Run("Empty body", func(t *testing.T) {
		status, _ := testPost(t, app, "/v1/wheel/spin", []byte("{}"), map[string]string{
			"Content-Type": "application/json",
		})

		assert.Equal(t, 400, status, "Should return 400 for empty body")
	})
}
