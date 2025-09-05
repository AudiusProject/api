package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// getCoinByField queries for a coin by a specific field (mint or ticker)
func (app *ApiServer) getCoinByField(c *fiber.Ctx, fieldName, fieldValue string) error {
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
		WHERE ` + fieldName + ` = @value
		LIMIT 1
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"value": fieldValue,
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

func (app *ApiServer) v1Coin(c *fiber.Ctx) error {
	mint := c.Params("mint")
	if mint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mint parameter is required",
		})
	}

	return app.getCoinByField(c, "artist_coins.mint", mint)
}

func (app *ApiServer) v1CoinByTicker(c *fiber.Ctx) error {
	ticker := c.Params("ticker")
	if ticker == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "ticker parameter is required",
		})
	}

	return app.getCoinByField(c, "artist_coins.ticker", ticker)
}
