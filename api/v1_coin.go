package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

const sharedSql = `
		SELECT
			artist_coins.name,
			artist_coins.mint,
			artist_coins.ticker,
			artist_coins.decimals,
			artist_coins.user_id,
			artist_coins.logo_uri,
			artist_coins.description,
			artist_coins.link_1,
			artist_coins.link_2,
			artist_coins.link_3,
			artist_coins.link_4,
			artist_coins.has_discord,
			artist_coins.created_at,
			artist_coins.updated_at AS coin_updated_at,
			COALESCE(artist_coin_stats.market_cap, 0) AS market_cap,
			COALESCE(artist_coin_stats.fdv, 0) AS fdv,
			COALESCE(artist_coin_stats.liquidity, 0) AS liquidity,
			COALESCE(artist_coin_stats.last_trade_unix_time, 0) AS last_trade_unix_time,
			COALESCE(artist_coin_stats.last_trade_human_time, '') AS last_trade_human_time,
			COALESCE(artist_coin_stats.price, 0) AS price,
			COALESCE(artist_coin_stats.history_24h_price, 0) AS history_24h_price,
			COALESCE(artist_coin_stats.price_change_24h_percent, 0) AS price_change_24h_percent,
			COALESCE(artist_coin_stats.unique_wallet_24h, 0) AS unique_wallet_24h,
			COALESCE(artist_coin_stats.unique_wallet_history_24h, 0) AS unique_wallet_history_24h,
			COALESCE(artist_coin_stats.unique_wallet_24h_change_percent, 0) AS unique_wallet_24h_change_percent,
			COALESCE(artist_coin_stats.total_supply, 0) AS total_supply,
			COALESCE(artist_coin_stats.circulating_supply, 0) AS circulating_supply,
			COALESCE(artist_coin_stats.holder, 0) AS holder,
			COALESCE(artist_coin_stats.trade_24h, 0) AS trade_24h,
			COALESCE(artist_coin_stats.trade_history_24h, 0) AS trade_history_24h,
			COALESCE(artist_coin_stats.trade_24h_change_percent, 0) AS trade_24h_change_percent,
			COALESCE(artist_coin_stats.sell_24h, 0) AS sell_24h,
			COALESCE(artist_coin_stats.sell_history_24h, 0) AS sell_history_24h,
			COALESCE(artist_coin_stats.sell_24h_change_percent, 0) AS sell_24h_change_percent,
			COALESCE(artist_coin_stats.buy_24h, 0) AS buy_24h,
			COALESCE(artist_coin_stats.buy_history_24h, 0) AS buy_history_24h,
			COALESCE(artist_coin_stats.buy_24h_change_percent, 0) AS buy_24h_change_percent,
			COALESCE(artist_coin_stats.v_24h, 0) AS v_24h,
			COALESCE(artist_coin_stats.v_24h_usd, 0) AS v_24h_usd,
			COALESCE(artist_coin_stats.v_history_24h, 0) AS v_history_24h,
			COALESCE(artist_coin_stats.v_history_24h_usd, 0) AS v_history_24h_usd,
			COALESCE(artist_coin_stats.v_24h_change_percent, 0) AS v_24h_change_percent,
			COALESCE(artist_coin_stats.v_buy_24h, 0) AS v_buy_24h,
			COALESCE(artist_coin_stats.v_buy_24h_usd, 0) AS v_buy_24h_usd,
			COALESCE(artist_coin_stats.v_buy_history_24h, 0) AS v_buy_history_24h,
			COALESCE(artist_coin_stats.v_buy_history_24h_usd, 0) AS v_buy_history_24h_usd,
			COALESCE(artist_coin_stats.v_buy_24h_change_percent, 0) AS v_buy_24h_change_percent,
			COALESCE(artist_coin_stats.v_sell_24h, 0) AS v_sell_24h,
			COALESCE(artist_coin_stats.v_sell_24h_usd, 0) AS v_sell_24h_usd,
			COALESCE(artist_coin_stats.v_sell_history_24h, 0) AS v_sell_history_24h,
			COALESCE(artist_coin_stats.v_sell_history_24h_usd, 0) AS v_sell_history_24h_usd,
			COALESCE(artist_coin_stats.v_sell_24h_change_percent, 0) AS v_sell_24h_change_percent,
			COALESCE(artist_coin_stats.number_markets, 0) AS number_markets,
			JSON_BUILD_OBJECT(
				'address', COALESCE(artist_coin_pools.address, ''),
				'price', COALESCE(artist_coin_pools.price, 0),
				'priceUSD', COALESCE(artist_coin_pools.price_usd, 0),
				'curveProgress', COALESCE(artist_coin_pools.curve_progress, 0),
				'isMigrated', COALESCE(artist_coin_pools.is_migrated, false),
				'creatorQuoteFee', COALESCE(artist_coin_pools.creator_quote_fee, 0),
				'totalTradingQuoteFee', COALESCE(artist_coin_pools.total_trading_quote_fee, 0),
				'creatorWalletAddress', COALESCE(artist_coin_pools.creator_wallet_address, '')
			) AS dynamic_bonding_curve,
			ROW_TO_JSON(calculate_artist_coin_fees(artist_coins.mint)) AS artist_fees,
			COALESCE(artist_coin_stats.updated_at, artist_coins.created_at) AS updated_at
		FROM artist_coins
		LEFT JOIN artist_coin_stats
			ON artist_coin_stats.mint = artist_coins.mint
		LEFT JOIN artist_coin_pools
			ON artist_coin_pools.base_mint = artist_coins.mint
`

func (app *ApiServer) v1Coin(c *fiber.Ctx) error {
	mint := c.Params("mint")
	if mint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mint parameter is required",
		})
	}

	sql := `
		` + sharedSql + `
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
		` + sharedSql + `
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
