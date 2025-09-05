package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1Coin(c *fiber.Ctx) error {
	mint := c.Params("mint")
	if mint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mint parameter is required",
		})
	}

	sql := `
		SELECT
			artist_coins.name,
			artist_coins.ticker,
			artist_coins.mint,
			artist_coins.decimals,
			artist_coins.user_id,
			artist_coins.logo_uri,
			artist_coins.description,
			artist_coins.website,
			artist_coins.created_at
		FROM artist_coins
		WHERE artist_coins.mint = @mint
		LIMIT 1
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"mint": mint,
	})
	if err != nil {
		return err
	}

	coinRow, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[ArtistCoin])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": coinRow,
	})
}

func (app *ApiServer) v1CoinByTicker(c *fiber.Ctx) error {
	ticker := c.Params("ticker")
	if ticker == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ticker parameter is required",
		})
	}

	sql := `
		SELECT
			artist_coins.name,
			artist_coins.ticker,
			artist_coins.mint,
			artist_coins.decimals,
			artist_coins.user_id,
			artist_coins.logo_uri,
			artist_coins.description,
			artist_coins.website,
			artist_coins.created_at
		FROM artist_coins
		WHERE artist_coins.ticker = @ticker
		LIMIT 1
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"ticker": ticker,
	})
	if err != nil {
		return err
	}

	coinRow, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[ArtistCoin])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": coinRow,
	})
}
