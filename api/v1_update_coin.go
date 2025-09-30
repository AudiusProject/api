package api

import (
	"errors"
	"log"
	"net/url"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type UpdateCoinBody struct {
	Description     string  `json:"description" validate:"max=2500"`
	XHandle         *string `json:"x_handle,omitempty"`
	InstagramHandle *string `json:"instagram_handle,omitempty"`
	TiktokHandle    *string `json:"tiktok_handle,omitempty"`
	Website         *string `json:"website,omitempty"`
}

func validateURL(s string) error {
	u, err := url.Parse(s)
	if err != nil {
		return err
	}
	if u.Scheme == "" || u.Host == "" {
		return errors.New("invalid URL format")
	}
	return nil
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
	hasUpdates := false

	if body.Description != "" {
		setParts = append(setParts, "description = @description")
		args["description"] = body.Description
		hasUpdates = true
	}
	if body.XHandle != nil {
		setParts = append(setParts, "x_handle = @x_handle")
		if *body.XHandle == "" {
			args["x_handle"] = nil
		} else {
			args["x_handle"] = *body.XHandle
		}
		hasUpdates = true
	}
	if body.InstagramHandle != nil {
		setParts = append(setParts, "instagram_handle = @instagram_handle")
		if *body.InstagramHandle == "" {
			args["instagram_handle"] = nil
		} else {
			args["instagram_handle"] = *body.InstagramHandle
		}
		hasUpdates = true
	}
	if body.TiktokHandle != nil {
		setParts = append(setParts, "tiktok_handle = @tiktok_handle")
		if *body.TiktokHandle == "" {
			args["tiktok_handle"] = nil
		} else {
			args["tiktok_handle"] = *body.TiktokHandle
		}
		hasUpdates = true
	}
	if body.Website != nil {
		if *body.Website != "" {
			// Validate URL format for non-empty values
			if err := validateURL(*body.Website); err != nil {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid website URL format")
			}
		}
		setParts = append(setParts, "website = @website")
		if *body.Website == "" {
			args["website"] = nil
		} else {
			args["website"] = *body.Website
		}
		hasUpdates = true
	}

	if !hasUpdates {
		return fiber.NewError(fiber.StatusBadRequest, "At least one field must be provided for update")
	}

	sql := `
		UPDATE artist_coins
		SET ` + strings.Join(setParts, ", ") + `
		WHERE mint = @mint
	`

	_, err = app.writePool.Exec(c.Context(), sql, args)
	if err != nil {
		log.Println("Failed to update coin", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to update coin",
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
	})
}
