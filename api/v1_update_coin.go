package api

import (
	"errors"
	"log"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type UpdateCoinBody struct {
	Description     string `json:"description" validate:"max=2500"`
	XHandle         string `json:"x_handle" validate:"omitempty,url"`
	InstagramHandle string `json:"instagram_handle" validate:"omitempty,url"`
	TiktokHandle    string `json:"tiktok_handle" validate:"omitempty,url"`
	Website         string `json:"website" validate:"omitempty,url"`
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
	if body.XHandle != "" {
		setParts = append(setParts, "x_handle = @x_handle")
		args["x_handle"] = body.XHandle
	}
	if body.InstagramHandle != "" {
		setParts = append(setParts, "instagram_handle = @instagram_handle")
		args["instagram_handle"] = body.InstagramHandle
	}
	if body.TiktokHandle != "" {
		setParts = append(setParts, "tiktok_handle = @tiktok_handle")
		args["tiktok_handle"] = body.TiktokHandle
	}
	if body.Website != "" {
		setParts = append(setParts, "website = @website")
		args["website"] = body.Website
	}

	sql := `
		UPDATE artist_coins
		SET ` + strings.Join(setParts, ", ") + `
		WHERE mint = @mint
		RETURNING mint, ticker, user_id, decimals, name, logo_uri, description, x_handle, instagram_handle, tiktok_handle, website, created_at, updated_at
	`

	row := app.writePool.QueryRow(c.Context(), sql, args)

	var result struct {
		Mint            string    `json:"mint"`
		Ticker          string    `json:"ticker"`
		UserID          int32     `json:"user_id"`
		Decimals        int32     `json:"decimals"`
		Name            string    `json:"name"`
		LogoUri         *string   `json:"logo_uri"`
		Description     *string   `json:"description"`
		XHandle         *string   `json:"x_handle"`
		InstagramHandle *string   `json:"instagram_handle"`
		TiktokHandle    *string   `json:"tiktok_handle"`
		Website         *string   `json:"website"`
		CreatedAt       time.Time `json:"created_at"`
		UpdatedAt       time.Time `json:"updated_at"`
	}

	if err := row.Scan(&result.Mint, &result.Ticker, &result.UserID, &result.Decimals, &result.Name, &result.LogoUri, &result.Description, &result.XHandle, &result.InstagramHandle, &result.TiktokHandle, &result.Website, &result.CreatedAt, &result.UpdatedAt); err != nil {
		log.Println("Failed to update coin", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update coin",
		})
	}

	return c.JSON(fiber.Map{
		"data": result,
	})
}
