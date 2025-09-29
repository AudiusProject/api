package api

import (
	"errors"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type UpdateCoinBody struct {
	Description string `json:"description" validate:"max=2500"`
	Twitter     string `json:"twitter" validate:"omitempty,url"`
	Instagram   string `json:"instagram" validate:"omitempty,url"`
	Tiktok      string `json:"tiktok" validate:"omitempty,url"`
	Website     string `json:"website" validate:"omitempty,url"`
}

func (app *ApiServer) v1UpdateCoin(c *fiber.Ctx) error {
	mint := c.Params("mint")
	if mint == "" {
		return fiber.NewError(fiber.StatusBadRequest, "Mint parameter is required")
	}

	body := UpdateCoinBody{}
	if err := app.ParseAndValidateBody(c, &body); err != nil {
		return err
	}

	userID := app.getMyId(c)

	// Check if user owns the coin
	var ownerID int32
	err := app.pool.QueryRow(c.Context(), `
		SELECT user_id FROM artist_coins
		WHERE mint = $1
	`, mint).Scan(&ownerID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "Coin not found")
		}
		return err
	}

	if ownerID != userID {
		return fiber.NewError(fiber.StatusForbidden, "You do not own this coin")
	}

	// Build dynamic UPDATE query based on provided fields
	setParts := []string{"updated_at = NOW()"}
	args := pgx.NamedArgs{"mint": mint}

	if body.Description != "" {
		setParts = append(setParts, "description = @description")
		args["description"] = body.Description
	}
	if body.Twitter != "" {
		setParts = append(setParts, "twitter = @twitter")
		args["twitter"] = body.Twitter
	}
	if body.Instagram != "" {
		setParts = append(setParts, "instagram = @instagram")
		args["instagram"] = body.Instagram
	}
	if body.Tiktok != "" {
		setParts = append(setParts, "tiktok = @tiktok")
		args["tiktok"] = body.Tiktok
	}
	if body.Website != "" {
		setParts = append(setParts, "website = @website")
		args["website"] = body.Website
	}

	sql := `
		UPDATE artist_coins
		SET ` + strings.Join(setParts, ", ") + `
		WHERE mint = @mint
		RETURNING mint, ticker, user_id, decimals, name, logo_uri, description, twitter, instagram, tiktok, website, created_at, updated_at
	`

	row := app.writePool.QueryRow(c.Context(), sql, args)

	var result struct {
		Mint        string    `json:"mint"`
		Ticker      string    `json:"ticker"`
		UserID      int32     `json:"user_id"`
		Decimals    int32     `json:"decimals"`
		Name        string    `json:"name"`
		LogoUri     *string   `json:"logo_uri"`
		Description *string   `json:"description"`
		Twitter     *string   `json:"twitter"`
		Instagram   *string   `json:"instagram"`
		Tiktok      *string   `json:"tiktok"`
		Website     *string   `json:"website"`
		CreatedAt   time.Time `json:"created_at"`
		UpdatedAt   time.Time `json:"updated_at"`
	}

	if err := row.Scan(&result.Mint, &result.Ticker, &result.UserID, &result.Decimals, &result.Name, &result.LogoUri, &result.Description, &result.Twitter, &result.Instagram, &result.Tiktok, &result.Website, &result.CreatedAt, &result.UpdatedAt); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update coin",
		})
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
