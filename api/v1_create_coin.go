package api

import (
	"errors"
	"fmt"
	"time"

	"api.audius.co/config"
	"api.audius.co/jobs"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type CreateCoinBody struct {
	Mint           string  `json:"mint" validate:"required,solana_address"`
	Ticker         string  `json:"ticker" validate:"required,min=2,max=10,coin_ticker"`
	Decimals       int32   `json:"decimals" validate:"required,min=0,max=18"`
	Name           string  `json:"name" validate:"required,min=1,max=32"`
	LogoUri        string  `json:"logo_uri" validate:"omitempty,url"`
	BannerImageUrl string  `json:"banner_image_url" validate:"omitempty,url"`
	Description    string  `json:"description" validate:"max=2500"`
	Link1          *string `json:"link_1,omitempty"`
	Link2          *string `json:"link_2,omitempty"`
	Link3          *string `json:"link_3,omitempty"`
	Link4          *string `json:"link_4,omitempty"`
}

func (app *ApiServer) v1CreateCoin(c *fiber.Ctx) error {
	body := CreateCoinBody{}
	if err := app.ParseAndValidateBody(c, &body); err != nil {
		return err
	}

	userID := app.getMyId(c)

	// Check if user is verified and active
	var isVerified bool
	err := app.pool.QueryRow(c.Context(), `
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

	linkValues := map[string]interface{}{
		"link_1": nil,
		"link_2": nil,
		"link_3": nil,
		"link_4": nil,
	}

	linkInputs := []struct {
		field string
		value *string
	}{
		{"link_1", body.Link1},
		{"link_2", body.Link2},
		{"link_3", body.Link3},
		{"link_4", body.Link4},
	}

	for _, link := range linkInputs {
		if link.value == nil {
			continue
		}
		if *link.value != "" {
			if err := validateURL(*link.value); err != nil {
				return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("Invalid %s URL format", link.field))
			}
			linkValues[link.field] = *link.value
		} else {
			linkValues[link.field] = nil
		}
	}

	sql := `
		INSERT INTO artist_coins (mint, ticker, user_id, decimals, name, logo_uri, banner_image_url, description, link_1, link_2, link_3, link_4)
		VALUES (@mint, @ticker, @user_id, @decimals, @name, @logo_uri, @banner_image_url, @description, @link_1, @link_2, @link_3, @link_4)
		RETURNING mint, ticker, user_id, decimals, name, logo_uri, banner_image_url, description, link_1, link_2, link_3, link_4, created_at
	`

	args := pgx.NamedArgs{
		"mint":             body.Mint,
		"ticker":           body.Ticker,
		"user_id":          userID,
		"decimals":         body.Decimals,
		"name":             body.Name,
		"logo_uri":         body.LogoUri,
		"banner_image_url": body.BannerImageUrl,
		"description":      body.Description,
		"link_1":           linkValues["link_1"],
		"link_2":           linkValues["link_2"],
		"link_3":           linkValues["link_3"],
		"link_4":           linkValues["link_4"],
	}

	row := app.writePool.QueryRow(c.Context(), sql, args)

	var result struct {
		Mint           string    `json:"mint"`
		Ticker         string    `json:"ticker"`
		UserID         int32     `json:"user_id"`
		Decimals       int32     `json:"decimals"`
		Name           string    `json:"name"`
		LogoUri        string    `json:"logo_uri"`
		BannerImageUrl *string   `json:"banner_image_url,omitempty"`
		Description    string    `json:"description"`
		Link1          *string   `json:"link_1,omitempty"`
		Link2          *string   `json:"link_2,omitempty"`
		Link3          *string   `json:"link_3,omitempty"`
		Link4          *string   `json:"link_4,omitempty"`
		CreatedAt      time.Time `json:"created_at"`
	}

	if err := row.Scan(&result.Mint, &result.Ticker, &result.UserID, &result.Decimals, &result.Name, &result.LogoUri, &result.BannerImageUrl, &result.Description, &result.Link1, &result.Link2, &result.Link3, &result.Link4, &result.CreatedAt); err != nil {
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

	// Update the pool for the new coin
	coinJob := jobs.NewCoinDBCJob(config.Cfg, app.writePool)
	coinJob.UpdatePoolForBaseMint(c.Context(), result.Mint, true)

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{
		"data": result,
	})
}
