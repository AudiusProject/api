package api

import (
	"errors"
	"fmt"

	"github.com/gagliardetto/solana-go"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type AccountBalance struct {
	Account       string  `json:"account"`
	Owner         string  `json:"owner"`
	Balance       float64 `json:"balance"`
	BalanceUSD    float64 `json:"balance_usd"`
	IsInAppWallet bool    `json:"is_in_app_wallet"`
}

type UserCoinAccounts struct {
	UserCoin
	Accounts []AccountBalance `json:"accounts"`
}

type GetUsersCoinRouteParams struct {
	Mint string `params:"mint"`
}

func (app *ApiServer) v1UsersCoin(c *fiber.Ctx) error {
	params := GetUsersCoinRouteParams{}
	if err := c.ParamsParser(&params); err != nil {
		return err
	}

	_, err := solana.PublicKeyFromBase58(params.Mint)
	if err != nil {
		return fiber.NewError(fiber.StatusBadRequest, fmt.Sprintf("invalid mint address: %s", params.Mint))
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
				AND user_bank_balances.mint = @mint
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
				AND associated_wallet_balances.mint = @mint
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
			artist_coins.logo_uri,
			artist_coins.banner_image_url,
			COALESCE(balances_by_mint.balance, 0) AS balance,
			COALESCE((balances_by_mint.balance * COALESCE(acp.price, 0)) / POWER(10, artist_coins.decimals), 0) AS balance_usd,
			COALESCE(
				JSON_AGG(
					JSON_BUILD_OBJECT(
						'account', balances.account,
						'owner', balances.owner,
						'balance', balances.balance,
						'balance_usd', (balances.balance * COALESCE(acp.price, 0)) / POWER(10, artist_coins.decimals),
						'is_in_app_wallet', balances.is_in_app_wallet
					)
				) FILTER (WHERE balances.account IS NOT NULL),
				'[]'::json
			) AS accounts
		FROM artist_coins
		LEFT JOIN artist_coin_prices acp ON acp.mint = artist_coins.mint
		LEFT JOIN balances_by_mint ON artist_coins.mint = balances_by_mint.mint
		LEFT JOIN (
			SELECT *
			FROM balances
			ORDER BY balances.balance DESC
		) AS balances ON balances.mint = balances_by_mint.mint
		WHERE artist_coins.mint = @mint
		GROUP BY
			artist_coins.ticker,
			artist_coins.mint,
			artist_coins.decimals,
			artist_coins.user_id,
			artist_coins.logo_uri,
			artist_coins.banner_image_url,
			balances_by_mint.balance,
			acp.price
	;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id": app.getUserId(c),
		"mint":    params.Mint,
	})
	if err != nil {
		return err
	}

	userCoin, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[UserCoinAccounts])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return c.JSON(fiber.Map{
				"data": nil,
			})
		}
		return err
	}

	return c.JSON(fiber.Map{
		"data": userCoin,
	})
}
