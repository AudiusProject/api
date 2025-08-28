package api

import (
	"bridgerton.audius.co/api/birdeye"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type ArtistCoinInsights struct {
	birdeye.TokenOverview
	MembersStatsRow
}

type MembersStatsRow struct {
	Members int `json:"members"`
}

func (app *ApiServer) v1CoinInsights(c *fiber.Ctx) error {
	mint := c.Params("mint")
	if mint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mint parameter is required",
		})
	}

	sql := `
		SELECT COUNT(DISTINCT user_id) AS members
		FROM sol_user_balances
		WHERE mint = @mint AND balance > 0
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"mint": mint,
	})
	if err != nil {
		return err
	}

	membersStatsRows, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[MembersStatsRow])
	if err != nil {
		return err
	}

	overview, err := app.birdeyeClient.GetTokenOverview(c.Context(), mint, "24h")
	if err != nil {
		return err
	}
	if overview == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Token overview not found",
		})
	}

	insights := ArtistCoinInsights{
		MembersStatsRow: membersStatsRows,
		TokenOverview:   *overview,
	}

	return c.JSON(fiber.Map{
		"data": insights,
	})
}
