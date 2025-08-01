package api

import (
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUserIdQueryParams struct {
	Addresses []string `query:"address"`
}

func (app *ApiServer) v1UserIdsByAddresses(c *fiber.Ctx) error {
	queryParams := GetUserIdQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}

	sql := `
		-- User account wallets
		SELECT
			users.user_id,
			users.wallet AS address
		FROM users
		WHERE users.wallet = ANY(@addresses) 
			AND users.is_deactivated = FALSE
			AND users.is_current = TRUE

		UNION ALL

		-- Associated wallets
		SELECT 
			associated_wallets.user_id,
			associated_wallets.wallet AS address
		FROM associated_wallets
		WHERE associated_wallets.wallet = ANY(@addresses)
			AND associated_wallets.is_current = TRUE
			AND associated_wallets.is_delete = FALSE

		UNION ALL

		-- User bank accounts
		SELECT 
			users.user_id, 
			sol_claimable_accounts.account AS address	
		FROM sol_claimable_accounts
		JOIN users 
			ON users.wallet = sol_claimable_accounts.ethereum_address
		WHERE sol_claimable_accounts.account = ANY(@addresses)
	;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"addresses": queryParams.Addresses,
	})
	if err != nil {
		return err
	}

	type addressRow struct {
		UserId  trashid.HashId `db:"user_id" json:"user_id"`
		Address string         `db:"address" json:"address"`
	}

	res, err := pgx.CollectRows(rows, pgx.RowToStructByName[addressRow])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": res,
	})
}
