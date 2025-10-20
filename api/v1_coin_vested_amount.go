package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// v1CoinVestedAmount returns the amount of vested coins that can be claimed for a given artist coin.
// After an artist coin graduates (is_migrated = true), the artist's reserved coins unlock daily
// over a 5-year period (1,826 days).
//
// TODO: This endpoint currently returns placeholder data. To fully implement it, we need:
// 1. Add vesting-related fields to the database schema (e.g., reserved_amount, claimed_amount, graduation_date)
// 2. Implement the vesting calculation logic based on time elapsed since graduation
// 3. Query the actual vesting state from Solana (or cache it in the database)
func (app *ApiServer) v1CoinVestedAmount(c *fiber.Ctx) error {
	mint := c.Params("mint")
	if mint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mint parameter is required",
		})
	}

	// Query to check if the coin has graduated and get relevant pool information
	sql := `
		SELECT
			artist_coins.mint,
			artist_coins.ticker,
			artist_coins.user_id,
			artist_coin_pools.is_migrated,
			artist_coin_pools.created_at as pool_created_at,
			artist_coin_pools.updated_at as pool_updated_at
		FROM artist_coins
		LEFT JOIN artist_coin_pools
			ON artist_coin_pools.base_mint = artist_coins.mint
		WHERE artist_coins.mint = @mint
		LIMIT 1
	`

	type VestingQueryResult struct {
		Mint          string `db:"mint"`
		Ticker        string `db:"ticker"`
		UserId        int32  `db:"user_id"`
		IsMigrated    bool   `db:"is_migrated"`
		PoolCreatedAt string `db:"pool_created_at"`
		PoolUpdatedAt string `db:"pool_updated_at"`
	}

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"mint": mint,
	})
	if err != nil {
		return err
	}

	result, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[VestingQueryResult])
	if err != nil {
		return err
	}

	// Check if the coin has graduated
	if !result.IsMigrated {
		// Coin hasn't graduated yet, so no vested coins available
		return c.JSON(fiber.Map{
			"data": fiber.Map{
				"mint":                result.Mint,
				"ticker":              result.Ticker,
				"hasGraduated":        false,
				"vestedAmount":        "0",
				"totalReservedAmount": "0",
				"claimedAmount":       "0",
				"graduationDate":      nil,
			},
		})
	}

	// TODO: Calculate actual vested amount based on:
	// 1. Get the graduation date (when is_migrated became true)
	// 2. Get the total reserved amount for the artist (typically a percentage of total supply)
	// 3. Calculate days since graduation
	// 4. Calculate vested amount: (days_since_graduation / 1826) * total_reserved_amount
	// 5. Subtract already claimed amount
	// 6. Query Solana vesting account state for accurate data

	// Placeholder response
	// In a real implementation, these values would come from:
	// - Database fields storing vesting schedule info
	// - Solana vesting program account state
	// - Calculation based on time elapsed since graduation
	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"mint":                result.Mint,
			"ticker":              result.Ticker,
			"hasGraduated":        true,
			"vestedAmount":        "0",                  // Amount available to claim now (in lamports/smallest unit)
			"totalReservedAmount": "0",                  // Total reserved for the artist
			"claimedAmount":       "0",                  // Already claimed amount
			"graduationDate":      result.PoolUpdatedAt, // Approximate - should track exact migration time
			// Additional useful fields:
			"vestingDurationDays": 1826, // 5 years
			"vestingStartDate":    result.PoolUpdatedAt,
		},
	})
}
