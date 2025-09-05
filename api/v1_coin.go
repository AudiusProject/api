package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1Coin(c *fiber.Ctx) error {
	input := c.Params("identifier")
	if input == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "identifier parameter is required (either mint or ticker)",
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
		WHERE artist_coins.mint = @input
		   OR artist_coins.ticker = @input
		LIMIT 1
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"input": input,
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
