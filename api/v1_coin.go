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
			artist_coins.updated_at as coin_updated_at,
			COALESCE(artist_coin_stats.market_cap, 0) as market_cap,
			COALESCE(artist_coin_stats.fdv, 0) as fdv,
			COALESCE(artist_coin_stats.liquidity, 0) as liquidity,
			COALESCE(artist_coin_stats.last_trade_unix_time, 0) as last_trade_unix_time,
			COALESCE(artist_coin_stats.last_trade_human_time, '') as last_trade_human_time,
			COALESCE(artist_coin_stats.price, 0) as price,
			COALESCE(artist_coin_stats.history_24h_price, 0) as history_24h_price,
			COALESCE(artist_coin_stats.price_change_24h_percent, 0) as price_change_24h_percent,
			COALESCE(artist_coin_stats.unique_wallet_24h, 0) as unique_wallet_24h,
			COALESCE(artist_coin_stats.unique_wallet_history_24h, 0) as unique_wallet_history_24h,
			COALESCE(artist_coin_stats.unique_wallet_24h_change_percent, 0) as unique_wallet_24h_change_percent,
			COALESCE(artist_coin_stats.total_supply, 0) as total_supply,
			COALESCE(artist_coin_stats.circulating_supply, 0) as circulating_supply,
			COALESCE(artist_coin_stats.holder, 0) as holder,
			COALESCE(artist_coin_stats.trade_24h, 0) as trade_24h,
			COALESCE(artist_coin_stats.trade_history_24h, 0) as trade_history_24h,
			COALESCE(artist_coin_stats.trade_24h_change_percent, 0) as trade_24h_change_percent,
			COALESCE(artist_coin_stats.sell_24h, 0) as sell_24h,
			COALESCE(artist_coin_stats.sell_history_24h, 0) as sell_history_24h,
			COALESCE(artist_coin_stats.sell_24h_change_percent, 0) as sell_24h_change_percent,
			COALESCE(artist_coin_stats.buy_24h, 0) as buy_24h,
			COALESCE(artist_coin_stats.buy_history_24h, 0) as buy_history_24h,
			COALESCE(artist_coin_stats.buy_24h_change_percent, 0) as buy_24h_change_percent,
			COALESCE(artist_coin_stats.v_24h, 0) as v_24h,
			COALESCE(artist_coin_stats.v_24h_usd, 0) as v_24h_usd,
			COALESCE(artist_coin_stats.v_history_24h, 0) as v_history_24h,
			COALESCE(artist_coin_stats.v_history_24h_usd, 0) as v_history_24h_usd,
			COALESCE(artist_coin_stats.v_24h_change_percent, 0) as v_24h_change_percent,
			COALESCE(artist_coin_stats.v_buy_24h, 0) as v_buy_24h,
			COALESCE(artist_coin_stats.v_buy_24h_usd, 0) as v_buy_24h_usd,
			COALESCE(artist_coin_stats.v_buy_history_24h, 0) as v_buy_history_24h,
			COALESCE(artist_coin_stats.v_buy_history_24h_usd, 0) as v_buy_history_24h_usd,
			COALESCE(artist_coin_stats.v_buy_24h_change_percent, 0) as v_buy_24h_change_percent,
			COALESCE(artist_coin_stats.v_sell_24h, 0) as v_sell_24h,
			COALESCE(artist_coin_stats.v_sell_24h_usd, 0) as v_sell_24h_usd,
			COALESCE(artist_coin_stats.v_sell_history_24h, 0) as v_sell_history_24h,
			COALESCE(artist_coin_stats.v_sell_history_24h_usd, 0) as v_sell_history_24h_usd,
			COALESCE(artist_coin_stats.v_sell_24h_change_percent, 0) as v_sell_24h_change_percent,
			COALESCE(artist_coin_stats.number_markets, 0) as number_markets,
			COALESCE(artist_coin_stats.total_volume, 0) as total_volume,
			COALESCE(artist_coin_stats.total_volume_usd, 0) as total_volume_usd,
			COALESCE(artist_coin_stats.volume_buy, 0) as volume_buy,
			COALESCE(artist_coin_stats.volume_buy_usd, 0) as volume_buy_usd,
			COALESCE(artist_coin_stats.volume_sell, 0) as volume_sell,
			COALESCE(artist_coin_stats.volume_sell_usd, 0) as volume_sell_usd,
			COALESCE(artist_coin_stats.total_trade, 0) as total_trade,
			COALESCE(artist_coin_stats.buy, 0) as buy,
			COALESCE(artist_coin_stats.sell, 0) as sell,
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
