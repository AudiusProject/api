package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetCoinMembersCountRouteParams struct {
	Mint string `params:"mint"`
}

type GetCoinMembersCountQueryParams struct {
	MinBalance float64 `query:"min_balance" default:"1.0" validate:"min=0"`
}

func (app *ApiServer) v1CoinMembersCount(c *fiber.Ctx) error {
	params := GetCoinMembersCountRouteParams{}
	if err := c.ParamsParser(&params); err != nil {
		return err
	}

	queryParams := GetCoinMembersCountQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}

	sql := `
		SELECT COUNT(*)
		FROM sol_user_balances
		JOIN artist_coins ON artist_coins.mint = sol_user_balances.mint
		WHERE balance >= @min_balance * POWER(10, artist_coins.decimals)
			AND sol_user_balances.mint = @mint
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"mint":        params.Mint,
		"min_balance": queryParams.MinBalance,
	})
	if err != nil {
		return err
	}
	defer rows.Close()

	count, err := pgx.CollectExactlyOneRow(rows, pgx.RowTo[int])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": count,
	})
}
