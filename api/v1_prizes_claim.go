package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	mathrand "math/rand"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"
)

const (
	yakMintAddress       = "ZDaUDL4XFdEct7UgeztrFQAptsvh4ZdhyZDZ1RpxYAK"
	yakSpinAmount        = 2000000000 // 2 YAK with 9 decimals
	yakAirdropAmount     = 1000000000 // 1 YAK with 9 decimals
	prizeReceiverAddress = "EHd892m3xNWGBuAXnafavqcFjXXUZp9bGecdSDNP2SLR"
)

type PrizeClaimRequest struct {
	Signature string `json:"signature" validate:"required"`
	Wallet    string `json:"wallet" validate:"required"`
}

type PrizeClaimResponse struct {
	PrizeID    string          `json:"prize_id"`
	PrizeName  string          `json:"prize_name"`
	Wallet     string          `json:"wallet"`
	PrizeType  *string         `json:"prize_type,omitempty"`
	ActionData json.RawMessage `json:"action_data,omitempty"`
}

type ClaimedPrizeResponse struct {
	ID         int64           `json:"id"`
	Wallet     string          `json:"wallet"`
	Signature  string          `json:"signature"`
	Mint       string          `json:"mint"`
	Amount     int64           `json:"amount"`
	PrizeID    string          `json:"prize_id"`
	PrizeName  string          `json:"prize_name"`
	PrizeType  *string         `json:"prize_type,omitempty"`
	ActionData json.RawMessage `json:"action_data,omitempty"`
	CreatedAt  time.Time       `json:"created_at"`
}

type Prize struct {
	ID          string
	Name        string
	Description string
	Weight      int
	Metadata    pgtype.Text
}

type PrizeMetadata struct {
	Type        string          `json:"type"`
	Amount      int64           `json:"amount"`
	URL         string          `json:"url,omitempty"`
	DownloadURL string          `json:"download_url,omitempty"`
	ActionData  json.RawMessage `json:"action_data,omitempty"`
}

// PrizePublicResponse is used for public endpoints (like get_prizes for spinner UI)
// It excludes sensitive information like URLs that should only be revealed after claiming
type PrizePublicResponse struct {
	PrizeID     string                 `json:"prize_id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Weight      int                    `json:"weight"`
	PublicMeta  map[string]interface{} `json:"metadata,omitempty"` // Sanitized metadata without URLs
}

// sanitizePrizeMetadata removes sensitive fields (URLs) from metadata for public display
func sanitizePrizeMetadata(metadata pgtype.Text) map[string]interface{} {
	if !metadata.Valid {
		return nil
	}

	var fullMeta PrizeMetadata
	if err := json.Unmarshal([]byte(metadata.String), &fullMeta); err != nil {
		return nil
	}

	// Create sanitized metadata with only safe fields
	sanitized := make(map[string]interface{})
	if fullMeta.Type != "" {
		sanitized["type"] = fullMeta.Type
	}
	// Include amount for display purposes (e.g., "1 YAK") but not URLs
	if fullMeta.Amount > 0 {
		sanitized["amount"] = fullMeta.Amount
	}
	// Explicitly exclude: URL, DownloadURL, ActionData

	return sanitized
}

func (app *ApiServer) v1PrizesClaim(c *fiber.Ctx) error {
	var req PrizeClaimRequest
	if err := app.ParseAndValidateBody(c, &req); err != nil {
		return err
	}

	ctx := c.Context()

	// Check if this signature has already been used
	var alreadyUsed bool
	err := app.pool.QueryRow(ctx, `
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
	// We need to find balance changes where:
	// 1. The signature matches
	// 2. The mint matches YAK
	// 3. The user's account has a negative change of exactly -2 YAK (spending)
	// 4. The prize receiver's account has a positive change of exactly +2 YAK (receiving)
	var userBalanceChange struct {
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
		&userBalanceChange.Owner,
		&userBalanceChange.Change,
		&userBalanceChange.Account,
	)

	if queryErr != nil {
		if errors.Is(queryErr, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusBadRequest, "Transaction not found or invalid. Must be exactly 2 YAK sent to the prize address.")
		}
		app.logger.Error("Failed to query balance changes", zap.Error(queryErr))
		return fiber.NewError(fiber.StatusInternalServerError, "Database error")
	}

	// Verify the wallet matches the transaction owner
	if userBalanceChange.Owner != req.Wallet {
		return fiber.NewError(fiber.StatusBadRequest, "Wallet does not match transaction owner")
	}

	// Verify the transaction sent tokens to the prize receiver address
	var receiverBalanceChange struct {
		Owner  string
		Change int64
	}

	receiverQueryErr := app.pool.QueryRow(ctx, `
		SELECT owner, change
		FROM sol_token_account_balance_changes
		WHERE signature = $1
			AND mint = $2
			AND owner = $3
			AND change = $4
		LIMIT 1
	`, req.Signature, yakMintAddress, prizeReceiverAddress, yakSpinAmount).Scan(
		&receiverBalanceChange.Owner,
		&receiverBalanceChange.Change,
	)

	if receiverQueryErr != nil {
		if errors.Is(receiverQueryErr, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusBadRequest, "Transaction not found or invalid. Must send exactly 2 YAK to the prize receiver address.")
		}
		app.logger.Error("Failed to query receiver balance changes", zap.Error(receiverQueryErr))
		return fiber.NewError(fiber.StatusInternalServerError, "Database error")
	}

	// Get all active prizes
	prizeRows, err := app.pool.Query(ctx, `
		SELECT prize_id, name, COALESCE(description, ''), weight, metadata
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
		err := prizeRows.Scan(&prize.ID, &prize.Name, &prize.Description, &prize.Weight, &prize.Metadata)
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
	rng := mathrand.New(mathrand.NewSource(time.Now().UnixNano()))
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

	// Check if this is a coin airdrop prize and generate redeem code if needed
	var prizeType *string
	var actionData json.RawMessage
	if selectedPrize.Metadata.Valid {
		var metadata PrizeMetadata
		if err := json.Unmarshal([]byte(selectedPrize.Metadata.String), &metadata); err == nil {
			if metadata.Type == "coin_airdrop" && metadata.Amount == yakAirdropAmount {
				code, url, err := app.generateRedeemCodeForPrize(ctx, yakMintAddress, yakAirdropAmount)
				if err != nil {
					app.logger.Error("Failed to generate redeem code", zap.Error(err))
					return fiber.NewError(fiber.StatusInternalServerError, "Failed to generate redeem code")
				}
				prizeTypeStr := "coin_airdrop"
				prizeType = &prizeTypeStr
				actionData, _ = json.Marshal(map[string]string{
					"code": code,
					"url":  url,
				})
			} else if metadata.Type != "" {
				// For other prize types, use the type from metadata and pass through action_data
				prizeTypeStr := metadata.Type
				prizeType = &prizeTypeStr
				// If metadata has action_data, use it; otherwise construct from metadata
				if metadata.ActionData != nil {
					actionData = metadata.ActionData
				} else {
					// Construct action_data from metadata fields
					actionDataMap := make(map[string]interface{})
					if metadata.URL != "" {
						actionDataMap["url"] = metadata.URL
					}
					if metadata.DownloadURL != "" {
						actionDataMap["download_url"] = metadata.DownloadURL
					}
					if len(actionDataMap) > 0 {
						actionData, _ = json.Marshal(actionDataMap)
					}
				}
			}
		}
	}

	// Save the result to the database
	sql := `
		INSERT INTO claimed_prizes (wallet, signature, mint, amount, prize_id, prize_name, prize_type, action_data)
		VALUES (@wallet, @signature, @mint, @amount, @prize_id, @prize_name, @prize_type, @action_data)
		RETURNING prize_id, prize_name, wallet, prize_type, action_data
	`

	rows, err := app.pool.Query(ctx, sql, pgx.NamedArgs{
		"wallet":      req.Wallet,
		"signature":   req.Signature,
		"mint":        yakMintAddress,
		"amount":      yakSpinAmount,
		"prize_id":    selectedPrize.ID,
		"prize_name":  selectedPrize.Name,
		"prize_type":  prizeType,
		"action_data": actionData,
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

func (app *ApiServer) generateRedeemCodeForPrize(ctx context.Context, mint string, amount int64) (string, string, error) {
	// Generate a code (reuse the same generateCode function from v1_create_reward_code)
	code, err := generateCode()
	if err != nil {
		return "", "", fmt.Errorf("failed to generate code: %w", err)
	}

	// Use shared function to create reward code
	rewardAddress, err := app.createRewardCode(ctx, code, mint, amount, "Prize")
	if err != nil {
		return "", "", fmt.Errorf("failed to create reward code: %w", err)
	}

	// Insert the reward code into the database
	sql := `
		INSERT INTO reward_codes (code, mint, reward_address, amount, remaining_uses, signature)
		VALUES (@code, @mint, @reward_address, @amount, 1, @signature)
		ON CONFLICT (code) DO NOTHING
	`

	_, err = app.pool.Exec(ctx, sql, pgx.NamedArgs{
		"code":           code,
		"mint":           mint,
		"reward_address": rewardAddress,
		"amount":         amount,
		"signature":      "",
	})
	if err != nil {
		return "", "", fmt.Errorf("database error: %w", err)
	}

	// Get ticker for constructing redeem URL
	var ticker string
	err = app.pool.QueryRow(ctx, `
		SELECT ticker FROM artist_coins WHERE mint = $1 LIMIT 1
	`, mint).Scan(&ticker)
	if err != nil {
		// If ticker not found, use mint as fallback
		ticker = mint
	}

	// Construct redeem URL: /coins/{ticker}/redeem/{code}
	redeemURL := fmt.Sprintf("/coins/%s/redeem/%s", ticker, code)

	return code, redeemURL, nil
}

// v1Prizes returns a list of active prizes for the spinner UI
// This endpoint excludes sensitive information like download URLs
func (app *ApiServer) v1Prizes(c *fiber.Ctx) error {
	ctx := c.Context()

	// Get all active prizes
	prizeRows, err := app.pool.Query(ctx, `
		SELECT prize_id, name, COALESCE(description, ''), weight, metadata
		FROM prizes
		WHERE is_active = true
		ORDER BY id
	`)
	if err != nil {
		app.logger.Error("Failed to query prizes", zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError, "Database error")
	}
	defer prizeRows.Close()

	var prizes []PrizePublicResponse
	for prizeRows.Next() {
		var prize Prize
		err := prizeRows.Scan(&prize.ID, &prize.Name, &prize.Description, &prize.Weight, &prize.Metadata)
		if err != nil {
			app.logger.Error("Failed to scan prize", zap.Error(err))
			return fiber.NewError(fiber.StatusInternalServerError, "Database error")
		}

		publicPrize := PrizePublicResponse{
			PrizeID:     prize.ID,
			Name:        prize.Name,
			Description: prize.Description,
			Weight:      prize.Weight,
			PublicMeta:  sanitizePrizeMetadata(prize.Metadata),
		}
		prizes = append(prizes, publicPrize)
	}

	return c.JSON(fiber.Map{
		"data": prizes,
	})
}

// v1WalletPrizes returns all claimed prizes for a wallet
// Public endpoint - no authentication required
// Excludes sensitive action_data (download URLs, redeem codes) for security
// Users receive full action_data when claiming via /v1/prizes/claim
func (app *ApiServer) v1WalletPrizes(c *fiber.Ctx) error {
	ctx := c.Context()
	wallet := c.Params("wallet")
	if wallet == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Missing wallet parameter")
	}

	// Get all claimed prizes for this wallet
	// Note: We exclude action_data for security - sensitive URLs/codes are only returned during claim
	prizeRows, err := app.pool.Query(ctx, `
		SELECT id, wallet, signature, mint, amount, prize_id, prize_name, prize_type, created_at
		FROM claimed_prizes
		WHERE wallet = $1
		ORDER BY created_at DESC
	`, wallet)
	if err != nil {
		app.logger.Error("Failed to query claimed prizes", zap.Error(err))
		return fiber.NewError(fiber.StatusInternalServerError, "Database error")
	}
	defer prizeRows.Close()

	var prizes []ClaimedPrizeResponse
	for prizeRows.Next() {
		var prize ClaimedPrizeResponse
		var prizeType pgtype.Text
		err := prizeRows.Scan(
			&prize.ID,
			&prize.Wallet,
			&prize.Signature,
			&prize.Mint,
			&prize.Amount,
			&prize.PrizeID,
			&prize.PrizeName,
			&prizeType,
			&prize.CreatedAt,
		)
		if err != nil {
			app.logger.Error("Failed to scan claimed prize", zap.Error(err))
			return fiber.NewError(fiber.StatusInternalServerError, "Database error")
		}

		if prizeType.Valid {
			prize.PrizeType = &prizeType.String
		}
		// action_data is intentionally excluded for security
		// Users receive full action_data when claiming via /v1/prizes/claim

		prizes = append(prizes, prize)
	}

	return c.JSON(fiber.Map{
		"data": prizes,
	})
}
