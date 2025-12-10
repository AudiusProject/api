package api

import (
	"errors"
	"math/rand"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

const (
	yakMintAddress = "ZDaUDL4XFdEct7UgeztrFQAptsvh4ZdhyZDZ1RpxYAK"
	yakSpinAmount  = 250000000000 // 250 YAK with 9 decimals
)

type PrizeClaimRequest struct {
	Signature string `json:"signature" validate:"required"`
	Wallet    string `json:"wallet" validate:"required"`
}

type PrizeClaimResponse struct {
	PrizeID   string `json:"prize_id"`
	PrizeName string `json:"prize_name"`
	Wallet    string `json:"wallet"`
}

type Prize struct {
	ID          string
	Name        string
	Description string
	Weight      int
}

func (app *ApiServer) v1PrizesClaim(c *fiber.Ctx) error {
	var req PrizeClaimRequest
	if err := app.ParseAndValidateBody(c, &req); err != nil {
		return err
	}

	ctx := c.Context()

	// Check if this signature has already been used
	var alreadyUsed bool
	err := app.writePool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM claimed_prizes WHERE signature = $1)
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
	// 2. The mint matches YAK
	// 3. The owner matches the wallet (user spending from their account)
	// 4. The change is exactly -250 YAK (250000000000 with 9 decimals)
	var balanceChange struct {
		Owner   string
		Change  int64
		Account string
	}

	queryErr := app.pool.QueryRow(ctx, `
		SELECT owner, change, account
		FROM sol_token_account_balance_changes
		WHERE signature = $1
			AND mint = $2
			AND owner = $3
			AND change = $4
		LIMIT 1
	`, req.Signature, yakMintAddress, req.Wallet, -yakSpinAmount).Scan(
		&balanceChange.Owner,
		&balanceChange.Change,
		&balanceChange.Account,
	)

	if queryErr != nil {
		if errors.Is(queryErr, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusBadRequest, "Transaction not found or invalid. Must be exactly 250 YAK sent to the prize address.")
		}
		app.logger.Error("Failed to query balance changes", zap.Error(queryErr))
		return fiber.NewError(fiber.StatusInternalServerError, "Database error")
	}

	// Verify the wallet matches the transaction owner
	if balanceChange.Owner != req.Wallet {
		return fiber.NewError(fiber.StatusBadRequest, "Wallet does not match transaction owner")
	}

	// Get all active prizes
	// Use writePool to ensure we see the latest data (important in tests, and safe in production)
	prizeRows, err := app.writePool.Query(ctx, `
		SELECT prize_id, name, COALESCE(description, ''), weight
		FROM prizes
		WHERE is_active = true
		ORDER BY id
	`)
	if err != nil {
		app.logger.Error("Failed to query prizes", zap.Error(err))
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
		app.logger.Error("No prizes available")
		return fiber.NewError(fiber.StatusInternalServerError, "No prizes available")
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
		INSERT INTO claimed_prizes (wallet, signature, mint, amount, prize_id, prize_name)
		VALUES (@wallet, @signature, @mint, @amount, @prize_id, @prize_name)
		RETURNING prize_id, prize_name, wallet
	`

	rows, err := app.writePool.Query(ctx, sql, pgx.NamedArgs{
		"wallet":     req.Wallet,
		"signature":  req.Signature,
		"mint":       yakMintAddress,
		"amount":     yakSpinAmount,
		"prize_id":   selectedPrize.ID,
		"prize_name": selectedPrize.Name,
	})
	if err != nil {
		app.logger.Error("Failed to insert claimed prize", zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to save result")
	}

	response, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[PrizeClaimResponse])
	if err != nil {
		app.logger.Error("Failed to read response", zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError, "Failed to read response")
	}

	return c.JSON(response)
}
