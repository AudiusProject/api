package api

import (
	"errors"
	"math/rand"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type WheelSpinRequest struct {
	Signature string `json:"signature" validate:"required"`
	Wallet    string `json:"wallet" validate:"required"`
	Mint      string `json:"mint,omitempty"` // Optional: if not provided, will be inferred from transaction
}

type WheelSpinResponse struct {
	PrizeID   string `json:"prize_id"`
	PrizeName string `json:"prize_name"`
	Wallet    string `json:"wallet"`
	Mint      string `json:"mint"`
	Amount    int64  `json:"amount"` // Amount spent in smallest units
}

type Prize struct {
	ID          string
	Name        string
	Description string
	Weight      int
}

func (app *ApiServer) v1WheelSpin(c *fiber.Ctx) error {
	var req WheelSpinRequest
	if err := app.ParseAndValidateBody(c, &req); err != nil {
		return err
	}

	ctx := c.Context()

	// Check if this signature has already been used
	var alreadyUsed bool
	err := app.writePool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM wheel_spin_results WHERE signature = $1)
	`, req.Signature).Scan(&alreadyUsed)
	if err != nil {
		app.logger.Error("Failed to check if signature already used", zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError, "Database error")
	}
	if alreadyUsed {
		return fiber.NewError(fiber.StatusBadRequest, "Transaction signature already used")
	}

	// Verify the transaction in sol_token_account_balance_changes
	// We need to find a balance change where:
	// 1. The signature matches
	// 2. The mint matches (if provided) or any mint
	// 3. The owner matches the wallet (user spending from their account)
	// 4. The change is negative (spending tokens)
	var balanceChange struct {
		Owner   string
		Change  int64
		Mint    string
		Account string
	}

	var queryErr error
	if req.Mint != "" {
		// If mint is provided, filter by it
		queryErr = app.pool.QueryRow(ctx, `
			SELECT owner, change, mint, account
			FROM sol_token_account_balance_changes
			WHERE signature = $1
				AND mint = $2
				AND owner = $3
				AND change < 0
			ORDER BY ABS(change) DESC
			LIMIT 1
		`, req.Signature, req.Mint, req.Wallet).Scan(
			&balanceChange.Owner,
			&balanceChange.Change,
			&balanceChange.Mint,
			&balanceChange.Account,
		)
	} else {
		// If mint not provided, find any spend transaction for this wallet
		queryErr = app.pool.QueryRow(ctx, `
			SELECT owner, change, mint, account
			FROM sol_token_account_balance_changes
			WHERE signature = $1
				AND owner = $2
				AND change < 0
			ORDER BY ABS(change) DESC
			LIMIT 1
		`, req.Signature, req.Wallet).Scan(
			&balanceChange.Owner,
			&balanceChange.Change,
			&balanceChange.Mint,
			&balanceChange.Account,
		)
	}

	if queryErr != nil {
		if errors.Is(queryErr, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusBadRequest, "Transaction not found or invalid")
		}
		app.logger.Error("Failed to query balance changes", zap.Error(queryErr))
		return fiber.NewError(fiber.StatusInternalServerError, "Database error")
	}

	// Verify the wallet matches the transaction owner
	if balanceChange.Owner != req.Wallet {
		return fiber.NewError(fiber.StatusBadRequest, "Wallet does not match transaction owner")
	}

	// If mint was provided, verify it matches
	if req.Mint != "" && balanceChange.Mint != req.Mint {
		return fiber.NewError(fiber.StatusBadRequest, "Transaction mint does not match provided mint")
	}

	// Check if this mint+amount combination is a valid config and get config_id
	// Use writePool to ensure we see the latest data (important in tests, and safe in production)
	var configID int
	var configAmount int64
	err = app.writePool.QueryRow(ctx, `
		SELECT id, amount
		FROM wheel_spin_configs 
		WHERE mint = $1 
			AND amount = $2 
			AND is_active = true
	`, balanceChange.Mint, -balanceChange.Change).Scan(&configID, &configAmount)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			app.logger.Info("Invalid config",
				zap.String("signature", req.Signature),
				zap.String("mint", balanceChange.Mint),
				zap.Int64("amount", -balanceChange.Change),
			)
			return fiber.NewError(fiber.StatusBadRequest, "This coin and amount combination is not a valid wheel spin configuration")
		}
		app.logger.Error("Failed to query wheel spin configs", zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError, "Database error")
	}

	// Get active prizes for this config
	// Use writePool to ensure we see the latest data (important in tests, and safe in production)
	prizeRows, err := app.writePool.Query(ctx, `
		SELECT prize_id, name, COALESCE(description, ''), weight
		FROM wheel_spin_prizes
		WHERE config_id = $1 AND is_active = true
		ORDER BY id
	`, configID)
	if err != nil {
		app.logger.Error("Failed to query prizes", zap.Error(err), zap.Int("config_id", configID))
		return fiber.NewError(fiber.StatusInternalServerError, "Database error")
	}
	defer prizeRows.Close()

	var prizes []Prize
	var totalWeight int
	for prizeRows.Next() {
		var prize Prize
		err := prizeRows.Scan(&prize.ID, &prize.Name, &prize.Description, &prize.Weight)
		if err != nil {
			app.logger.Error("Failed to scan prize", zap.Error(err))
			return fiber.NewError(fiber.StatusInternalServerError, "Database error")
		}
		prizes = append(prizes, prize)
		totalWeight += prize.Weight
	}

	if len(prizes) == 0 {
		app.logger.Error("No prizes available for config", zap.Int("config_id", configID))
		return fiber.NewError(fiber.StatusInternalServerError, "No prizes available for this configuration")
	}

	// Weighted random selection
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	randomValue := rng.Intn(totalWeight)
	var selectedPrize Prize
	currentWeight := 0
	for _, prize := range prizes {
		currentWeight += prize.Weight
		if randomValue < currentWeight {
			selectedPrize = prize
			break
		}
	}

	// Save the result to the database
	sql := `
		INSERT INTO wheel_spin_results (wallet, signature, mint, amount, prize_id, prize_name)
		VALUES (@wallet, @signature, @mint, @amount, @prize_id, @prize_name)
		RETURNING prize_id, prize_name, wallet, mint, amount
	`

	rows, err := app.writePool.Query(ctx, sql, pgx.NamedArgs{
		"wallet":     req.Wallet,
		"signature":  req.Signature,
		"mint":       balanceChange.Mint,
		"amount":     configAmount, // Use amount from config
		"prize_id":   selectedPrize.ID,
		"prize_name": selectedPrize.Name,
	})
	if err != nil {
		app.logger.Error("Failed to insert wheel spin result", zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to save result")
	}

	response, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[WheelSpinResponse])
	if err != nil {
		app.logger.Error("Failed to read response", zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to read response")
	}

	return c.JSON(response)
}
