package api

import (
	"fmt"

	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersCoinsQueryParams struct {
	Limit  int `query:"limit" default:"50" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

type UserCoin struct {
	Ticker     string         `json:"ticker"`
	Mint       string         `json:"mint"`
	Decimals   int            `json:"decimals"`
	HasDiscord bool           `json:"has_discord"`
	OwnerID    trashid.HashId `json:"owner_id"`
	Balance    float64        `json:"balance"`
	BalanceUSD float64        `json:"balance_usd"`
}

func (app *ApiServer) v1UsersCoins(c *fiber.Ctx) error {
	queryParams := GetUsersCoinsQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}

	mintSql := `
		SELECT mint
		FROM artist_coins;
	`
	var mints []string
	rows, err := app.pool.Query(c.Context(), mintSql)
	if err != nil {
		return fmt.Errorf("failed to query mints: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var mint string
		if err := rows.Scan(&mint); err != nil {
			return fmt.Errorf("failed to scan mint: %w", err)
		}
		mints = append(mints, mint)
	}

	if len(mints) == 0 {
		return c.JSON(fiber.Map{
			"data": []UserCoin{},
		})
	}

	sql := `
		WITH balances AS (
			SELECT
				user_bank_balances.mint,
				user_bank_balances.balance AS balance,
				user_bank_balances.account AS account,
				user_bank_balances.owner AS owner,
				TRUE as is_in_app_wallet
			FROM users
			JOIN sol_claimable_accounts
				ON sol_claimable_accounts.ethereum_address = users.wallet
			JOIN sol_token_account_balances AS user_bank_balances
				ON user_bank_balances.account = sol_claimable_accounts.account
			WHERE users.user_id = @user_id
			UNION ALL
			SELECT
				associated_wallet_balances.mint,
				associated_wallet_balances.balance AS balance,
				associated_wallet_balances.account AS account,
				associated_wallet_balances.owner AS owner,
				FALSE as is_in_app_wallet
			FROM users
			JOIN associated_wallets
				ON associated_wallets.user_id = users.user_id
			JOIN sol_token_account_balances AS associated_wallet_balances
				ON associated_wallet_balances.owner = associated_wallets.wallet
				AND associated_wallets.chain = 'sol'
			WHERE associated_wallets.user_id = @user_id
		), balances_by_mint AS (
			SELECT
				balances.mint,
				SUM(balances.balance) AS balance
			FROM balances
			GROUP BY balances.mint
		)
		SELECT
			artist_coins.ticker,
			artist_coins.mint,
			artist_coins.decimals,
			artist_coins.has_discord,
			artist_coins.user_id AS owner_id,
			COALESCE(balances_by_mint.balance, 0) AS balance,
			(COALESCE(balances_by_mint.balance, 0) * stats.price) / POWER(10, artist_coins.decimals) AS balance_usd
		FROM artist_coins
		LEFT JOIN balances_by_mint ON balances_by_mint.mint = artist_coins.mint
		JOIN artist_coin_stats stats ON stats.mint = artist_coins.mint
		WHERE artist_coins.user_id = @user_id  -- Show owned coins
		   OR balance > 0  -- Show coins with positive balance
		ORDER BY
			-- Always show user's owned coins first, regardless of balance
			(artist_coins.user_id = @user_id) DESC,
			-- Then prioritize AUDIO
			artist_coins.ticker = '$AUDIO' DESC,
			-- Then by number of coins (balance)
			balance DESC,
			-- Finally by mint for consistent ordering
			artist_coins.mint ASC
		LIMIT @limit
		OFFSET @offset
	;`

	rows, err = app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id": app.getUserId(c),
		"limit":   queryParams.Limit,
		"offset":  queryParams.Offset,
	})
	if err != nil {
		return err
	}

	userCoins, err := pgx.CollectRows(rows, pgx.RowToStructByName[UserCoin])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": userCoins,
	})
}
