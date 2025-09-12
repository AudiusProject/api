package api

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type CreateCoinBody struct {
	Mint        string `json:"mint" validate:"required"`
	Ticker      string `json:"ticker" validate:"required"`
	Decimals    int32  `json:"decimals" validate:"required,min=0,max=18"`
	Name        string `json:"name" validate:"required,max=32"`
	LogoUri     string `json:"logo_uri"`
	Description string `json:"description" validate:"max=2500"`
}

func (app *ApiServer) v1CreateCoin(c *fiber.Ctx) error {
	body := CreateCoinBody{}
	err := c.BodyParser(&body)
	if err != nil {
		return err
	}
	if body.Mint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mint is required",
		})
	}
	if body.Ticker == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ticker is required",
		})
	}
	if body.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "name is required",
		})
	}

	userID := app.getMyId(c)

	// Check if user is verified and active
	var isVerified bool
	err = app.pool.QueryRow(c.Context(), `
		SELECT is_verified FROM users
		WHERE user_id = $1
			AND is_current = true
			AND is_deactivated = false
	`, userID).Scan(&isVerified)

	if err != nil {
		return err
	}
	if !isVerified {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "User must be verified to create coins",
		})
	}

	// Check if user has already created a coin
	var hasExistingCoin bool
	err = app.pool.QueryRow(c.Context(), `
		SELECT EXISTS(SELECT 1 FROM artist_coins WHERE user_id = $1)
	`, userID).Scan(&hasExistingCoin)

	if err != nil {
		return err
	}

	if hasExistingCoin {
		return fiber.NewError(fiber.StatusBadRequest, "User has already created a coin")
	}

	sql := `
		INSERT INTO artist_coins (mint, ticker, user_id, decimals, name, logo_uri, description)
		VALUES (@mint, @ticker, @user_id, @decimals, @name, @logo_uri, @description)
		RETURNING mint, ticker, user_id, decimals, name, logo_uri, description, created_at
	`

	row := app.writePool.QueryRow(c.Context(), sql, pgx.NamedArgs{
		"mint":        body.Mint,
		"ticker":      body.Ticker,
		"user_id":     userID,
		"decimals":    body.Decimals,
		"name":        body.Name,
		"logo_uri":    body.LogoUri,
		"description": body.Description,
	})

	var result struct {
		Mint        string    `json:"mint"`
		Ticker      string    `json:"ticker"`
		UserID      int32     `json:"user_id"`
		Decimals    int32     `json:"decimals"`
		Name        string    `json:"name"`
		LogoUri     string    `json:"logo_uri"`
		Description string    `json:"description"`
		CreatedAt   time.Time `json:"created_at"`
	}

	if err := row.Scan(&result.Mint, &result.Ticker, &result.UserID, &result.Decimals, &result.Name, &result.LogoUri, &result.Description, &result.CreatedAt); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgerrcode.UniqueViolation {
			if pgErr.ConstraintName == "artist_coins_pkey" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Mint already exists",
				})
			}
			if pgErr.ConstraintName == "artist_coins_ticker_unique" {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
					"error": "Ticker already exists",
				})
			}
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to create coin",
		})
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": result,
	})
}
