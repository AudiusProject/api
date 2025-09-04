package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type ArtistCoinStatsRow struct {
	Mint                         string                       `json:"mint" db:"mint"`
	MarketCap                    float64                      `json:"marketCap" db:"market_cap"`
	FDV                          float64                      `json:"fdv" db:"fdv"`
	Liquidity                    float64                      `json:"liquidity" db:"liquidity"`
	LastTradeUnixTime            int64                        `json:"lastTradeUnixTime" db:"last_trade_unix_time"`
	LastTradeHumanTime           string                       `json:"lastTradeHumanTime" db:"last_trade_human_time"`
	Price                        float64                      `json:"price" db:"price"`
	History24hPrice              float64                      `json:"history24hPrice" db:"history_24h_price"`
	PriceChange24hPercent        float64                      `json:"priceChange24hPercent" db:"price_change_24h_percent"`
	UniqueWallet24h              int                          `json:"uniqueWallet24h" db:"unique_wallet_24h"`
	UniqueWalletHistory24h       int                          `json:"uniqueWalletHistory24h" db:"unique_wallet_history_24h"`
	UniqueWallet24hChangePercent float64                      `json:"uniqueWallet24hChangePercent" db:"unique_wallet_24h_change_percent"`
	TotalSupply                  float64                      `json:"totalSupply" db:"total_supply"`
	CirculatingSupply            float64                      `json:"circulatingSupply" db:"circulating_supply"`
	Holder                       int                          `json:"holder" db:"holder"`
	Trade24h                     int                          `json:"trade24h" db:"trade_24h"`
	TradeHistory24h              int                          `json:"tradeHistory24h" db:"trade_history_24h"`
	Trade24hChangePercent        float64                      `json:"trade24hChangePercent" db:"trade_24h_change_percent"`
	Sell24h                      int                          `json:"sell24h" db:"sell_24h"`
	SellHistory24h               int                          `json:"sellHistory24h" db:"sell_history_24h"`
	Sell24hChangePercent         float64                      `json:"sell24hChangePercent" db:"sell_24h_change_percent"`
	Buy24h                       int                          `json:"buy24h" db:"buy_24h"`
	BuyHistory24h                int                          `json:"buyHistory24h" db:"buy_history_24h"`
	Buy24hChangePercent          float64                      `json:"buy24hChangePercent" db:"buy_24h_change_percent"`
	V24h                         float64                      `json:"v24h" db:"v_24h"`
	V24hUSD                      float64                      `json:"v24hUSD" db:"v_24h_usd"`
	VHistory24h                  float64                      `json:"vHistory24h" db:"v_history_24h"`
	VHistory24hUSD               float64                      `json:"vHistory24hUSD" db:"v_history_24h_usd"`
	V24hChangePercent            float64                      `json:"v24hChangePercent" db:"v_24h_change_percent"`
	VBuy24h                      float64                      `json:"vBuy24h" db:"v_buy_24h"`
	VBuy24hUSD                   float64                      `json:"vBuy24hUSD" db:"v_buy_24h_usd"`
	VBuyHistory24h               float64                      `json:"vBuyHistory24h" db:"v_buy_history_24h"`
	VBuyHistory24hUSD            float64                      `json:"vBuyHistory24hUSD" db:"v_buy_history_24h_usd"`
	VBuy24hChangePercent         float64                      `json:"vBuy24hChangePercent" db:"v_buy_24h_change_percent"`
	VSell24h                     float64                      `json:"vSell24h" db:"v_sell_24h"`
	VSell24hUSD                  float64                      `json:"vSell24hUSD" db:"v_sell_24h_usd"`
	VSellHistory24h              float64                      `json:"vSellHistory24h" db:"v_sell_history_24h"`
	VSellHistory24hUSD           float64                      `json:"vSellHistory24hUSD" db:"v_sell_history_24h_usd"`
	VSell24hChangePercent        float64                      `json:"vSell24hChangePercent" db:"v_sell_24h_change_percent"`
	NumberMarkets                int                          `json:"numberMarkets" db:"number_markets"`
	DynamicBondingCurve          *DynamicBondingCurveInsights `json:"dynamicBondingCurve" db:"dynamic_bonding_curve"`
	CreatedAt                    time.Time                    `json:"createdAt" db:"created_at"`
	UpdatedAt                    time.Time                    `json:"updatedAt" db:"updated_at"`
}

type MembersStatsRow struct {
	DbcPool *string `db:"dbc_pool" json:"-"`
	Members int     `json:"members"`
}

type DynamicBondingCurveInsights struct {
	Address       string  `json:"address"`
	Price         float64 `json:"price"`
	PriceUSD      float64 `json:"priceUSD"`
	CurveProgress float64 `json:"curveProgress"`
	IsMigrated    bool    `json:"isMigrated"`
}

func (app *ApiServer) v1CoinInsights(c *fiber.Ctx) error {
	mint := c.Params("mint")
	if mint == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "mint parameter is required",
		})
	}

	sql := `
		SELECT 
			artist_coin_stats.*,
			JSON_BUILD_OBJECT(
				'address', artist_coin_pools.address,
				'price', artist_coin_pools.price,
				'priceUSD', artist_coin_pools.price_usd,
				'curveProgress', artist_coin_pools.curve_progress,
				'isMigrated', artist_coin_pools.is_migrated
			) AS dynamic_bonding_curve
		FROM artist_coin_stats
		LEFT JOIN sol_user_balances 
			ON sol_user_balances.mint = artist_coin_stats.mint
			AND sol_user_balances.balance > 0
		LEFT JOIN artist_coin_pools
			ON artist_coin_pools.base_mint = artist_coin_stats.mint
		WHERE artist_coin_stats.mint = @mint
		LIMIT 1
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"mint": mint,
	})
	if err != nil {
		return err
	}

	artistCoinStatsRow, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[ArtistCoinStatsRow])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": artistCoinStatsRow,
	})
}
