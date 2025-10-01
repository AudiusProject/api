package api

import (
	"fmt"
	"time"

	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type ArtistCoin struct {
	Name          string         `json:"name"`
	Ticker        string         `json:"ticker"`
	Mint          string         `json:"mint"`
	Decimals      int            `json:"decimals"`
	OwnerId       trashid.HashId `db:"user_id" json:"owner_id"`
	LogoUri       *string        `json:"logo_uri,omitempty"`
	Description   *string        `json:"description,omitempty"`
	Link1         *string        `json:"link_1,omitempty"`
	Link2         *string        `json:"link_2,omitempty"`
	Link3         *string        `json:"link_3,omitempty"`
	Link4         *string        `json:"link_4,omitempty"`
	HasDiscord    bool           `json:"has_discord"`
	CreatedAt     time.Time      `json:"created_at"`
	CoinUpdatedAt time.Time      `json:"coin_updated_at"`

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
	UpdatedAt                    time.Time                    `json:"updatedAt" db:"updated_at"`
}

type GetArtistCoinsQueryParams struct {
	Tickers       []string         `query:"ticker"`
	Mints         []string         `query:"mint"`
	OwnerIds      []trashid.HashId `query:"owner_id"`
	Limit         int              `query:"limit" default:"50" validate:"min=1,max=100"`
	Offset        int              `query:"offset" default:"0" validate:"min=0"`
	Query         string           `query:"query"`
	SortMethod    string           `query:"sort_method" default:"market_cap" validate:"oneof=market_cap price volume created_at holder"`
	SortDirection string           `query:"sort_direction" default:"desc" validate:"oneof=asc desc"`
}

func (app *ApiServer) v1Coins(c *fiber.Ctx) error {
	queryParams := GetArtistCoinsQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &queryParams); err != nil {
		return err
	}

	mintFilter := ""
	if len(queryParams.Mints) > 0 {
		mintFilter = `AND artist_coins.mint = ANY(@mints)`
	}
	ownerIdFilter := ""
	if len(queryParams.OwnerIds) > 0 {
		ownerIdFilter = `AND artist_coins.user_id = ANY(@owner_ids)`
	}
	tickerFilter := ""
	if len(queryParams.Tickers) > 0 {
		tickerFilter = `AND artist_coins.ticker = ANY(@tickers)`
	}
	queryFilter := ""
	if queryParams.Query != "" {
		queryFilter = `AND (
			artist_coins.ticker ILIKE '%' || @query || '%' OR
			artist_coins.name ILIKE '%' || @query || '%' OR
			users.handle_lc ILIKE '%' || @query || '%'
		)`
	}

	sortMethod := "market_cap"
	switch queryParams.SortMethod {
	case "price":
		sortMethod = "price"
	case "volume":
		sortMethod = "v_24h_usd"
	case "created_at":
		sortMethod = "created_at"
	case "holder":
		sortMethod = "holder"
	}

	sortDirection := "desc"
	if queryParams.SortDirection == "asc" {
		sortDirection = "asc"
	}

	sortString := fmt.Sprintf("%s %s", sortMethod, sortDirection)

	sql := `
		SELECT
			artist_coins.name,
			artist_coins.ticker,
			artist_coins.mint,
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
			JSON_BUILD_OBJECT(
				'address', COALESCE(artist_coin_pools.address, ''),
				'price', COALESCE(artist_coin_pools.price, 0),
				'priceUSD', COALESCE(artist_coin_pools.price_usd, 0),
				'curveProgress', COALESCE(artist_coin_pools.curve_progress, 0),
				'isMigrated', COALESCE(artist_coin_pools.is_migrated, false),
				'creatorQuoteFee', COALESCE(artist_coin_pools.creator_quote_fee, 0),
				'totalTradingQuoteFee', COALESCE(artist_coin_pools.total_trading_quote_fee, 0)
			) AS dynamic_bonding_curve,
			COALESCE(artist_coin_stats.updated_at, artist_coins.created_at) as updated_at
		FROM artist_coins
		LEFT JOIN artist_coin_stats
			ON artist_coin_stats.mint = artist_coins.mint
		LEFT JOIN artist_coin_pools
			ON artist_coin_pools.base_mint = artist_coins.mint
		LEFT JOIN users
			ON users.user_id = artist_coins.user_id
		WHERE 1=1
			` + mintFilter + `
			` + ownerIdFilter + `
			` + tickerFilter + `
			` + queryFilter + `
		ORDER BY ` + sortString + `
		LIMIT @limit
		OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"tickers":   queryParams.Tickers,
		"mints":     queryParams.Mints,
		"owner_ids": queryParams.OwnerIds,
		"limit":     queryParams.Limit,
		"offset":    queryParams.Offset,
		"query":     queryParams.Query,
	})
	if err != nil {
		return err
	}

	coinRows, err := pgx.CollectRows(rows, pgx.RowToStructByNameLax[ArtistCoin])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": coinRows,
	})
}
