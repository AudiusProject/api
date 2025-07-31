package api

import (
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetCoinsMembersRouteParams struct {
	Mint string `params:"mint"`
}

type GetCoinsMembersQueryParams struct {
	MinBalance    int64  `query:"min_balance" default:"1" validate:"min=0"`
	SortDirection string `query:"sort_direction" default:"desc" validate:"oneof=asc desc"`
	Limit         int    `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset        int    `query:"offset" default:"0" validate:"min=0"`
}

type CoinMember struct {
	UserID  trashid.HashId `json:"user_id"`
	Balance int64          `json:"balance"`
}

func (app *ApiServer) v1CoinsMembers(c *fiber.Ctx) error {
	params := GetCoinsMembersRouteParams{}
	if err := c.ParamsParser(&params); err != nil {
		return err
	}

	queryParams := GetCoinsMembersQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}

	sortDirection := "DESC"
	if queryParams.SortDirection == "asc" {
		sortDirection = "ASC"
	}

	sql := `
		SELECT
			user_id,
			balance
		FROM sol_user_balances
		WHERE balance >= @min_balance
			AND mint = @mint
		ORDER BY 
			balance ` + sortDirection + `,
			user_id ASC
		LIMIT @limit
		OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"mint":           params.Mint,
		"min_balance":    queryParams.MinBalance,
		"sort_direction": queryParams.SortDirection,
		"limit":          queryParams.Limit,
		"offset":         queryParams.Offset,
	})
	if err != nil {
		return err
	}
	defer rows.Close()

	members, err := pgx.CollectRows(rows, pgx.RowToStructByName[CoinMember])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": members,
	})
}
