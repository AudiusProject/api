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

	// See v1_coins for the query explanation.
	sql := `
		SELECT 
			artist_coins.ticker,
			artist_coins.mint,
			artist_coins.decimals,
			artist_coins.user_id,
			artist_coins.created_at,
			123 as members, -- Placeholder for members count
			50 as members_24h_change_percent -- Placeholder for 24h change percent
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

	coinRows, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[ArtistCoin])
	if err != nil {
		return err
	}

	overview, err := app.birdeyeClient.GetTokenOverview(c.Context(), mint, "24h")
	if err != nil {
		return err
	}
	coinRows.TokenInfo = overview

	return c.JSON(fiber.Map{
		"data": coinRows,
	})
}
