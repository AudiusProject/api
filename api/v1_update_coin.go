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
	Description string  `json:"description" validate:"max=2500"`
	Link1       *string `json:"link_1,omitempty"`
	Link2       *string `json:"link_2,omitempty"`
	Link3       *string `json:"link_3,omitempty"`
	Link4       *string `json:"link_4,omitempty"`
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
	if body.Link1 != nil {
		if *body.Link1 != "" {
			// Validate URL format for non-empty values
			if err := validateURL(*body.Link1); err != nil {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid link_1 URL format")
			}
		}
		setParts = append(setParts, "link_1 = @link_1")
		if *body.Link1 == "" {
			args["link_1"] = nil
		} else {
			args["link_1"] = *body.Link1
		}
		hasUpdates = true
	}
	if body.Link2 != nil {
		if *body.Link2 != "" {
			// Validate URL format for non-empty values
			if err := validateURL(*body.Link2); err != nil {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid link_2 URL format")
			}
		}
		setParts = append(setParts, "link_2 = @link_2")
		if *body.Link2 == "" {
			args["link_2"] = nil
		} else {
			args["link_2"] = *body.Link2
		}
		hasUpdates = true
	}
	if body.Link3 != nil {
		if *body.Link3 != "" {
			// Validate URL format for non-empty values
			if err := validateURL(*body.Link3); err != nil {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid link_3 URL format")
			}
		}
		setParts = append(setParts, "link_3 = @link_3")
		if *body.Link3 == "" {
			args["link_3"] = nil
		} else {
			args["link_3"] = *body.Link3
		}
		hasUpdates = true
	}
	if body.Link4 != nil {
		if *body.Link4 != "" {
			// Validate URL format for non-empty values
			if err := validateURL(*body.Link4); err != nil {
				return fiber.NewError(fiber.StatusBadRequest, "Invalid link_4 URL format")
			}
		}
		setParts = append(setParts, "link_4 = @link_4")
		if *body.Link4 == "" {
			args["link_4"] = nil
		} else {
			args["link_4"] = *body.Link4
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
