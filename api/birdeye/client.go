package birdeye

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/maypok86/otter"
)

type Client struct {
	token string

	tokenOverviewCache otter.Cache[string, *TokenOverview]
}

func New(token string) *Client {
	tokenOverviewCache, err := otter.MustBuilder[string, *TokenOverview](100).
		WithTTL(10 * time.Second).
		CollectStats().
		Build()
	if err != nil {
		panic(err)
	}
	return &Client{
		token:              token,
		tokenOverviewCache: tokenOverviewCache,
	}
}

type TokenOverview struct {
	Address    string  `json:"address"`
	Decimals   int     `json:"decimals"`
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	MarketCap  float64 `json:"marketCap"`
	FDV        float64 `json:"fdv"`
	Extensions struct {
		CoingeckoID string `json:"coingeckoId"`
		Description string `json:"description"`
		Twitter     string `json:"twitter"`
		Website     string `json:"website"`
		Discord     string `json:"discord"`
	} `json:"extensions"`
	LogoURI                      string  `json:"logoURI"`
	Liquidity                    float64 `json:"liquidity"`
	LastTradeUnixTime            int64   `json:"lastTradeUnixTime"`
	LastTradeHumanTime           string  `json:"lastTradeHumanTime"`
	Price                        float64 `json:"price"`
	History24hPrice              float64 `json:"history24hPrice"`
	PriceChange24hPercent        float64 `json:"priceChange24hPercent"`
	UniqueWallet24h              int     `json:"uniqueWallet24h"`
	UniqueWalletHistory24h       int     `json:"uniqueWalletHistory24h"`
	UniqueWallet24hChangePercent float64 `json:"uniqueWallet24hChangePercent"`
	TotalSupply                  float64 `json:"totalSupply"`
	CirculatingSupply            float64 `json:"circulatingSupply"`
	Holder                       int     `json:"holder"`
	Trade24h                     int     `json:"trade24h"`
	TradeHistory24h              int     `json:"tradeHistory24h"`
	Trade24hChangePercent        float64 `json:"trade24hChangePercent"`
	Sell24h                      int     `json:"sell24h"`
	SellHistory24h               int     `json:"sellHistory24h"`
	Sell24hChangePercent         float64 `json:"sell24hChangePercent"`
	Buy24h                       int     `json:"buy24h"`
	BuyHistory24h                int     `json:"buyHistory24h"`
	Buy24hChangePercent          float64 `json:"buy24hChangePercent"`
	V24h                         float64 `json:"v24h"`
	V24hUSD                      float64 `json:"v24hUSD"`
	VHistory24h                  float64 `json:"vHistory24h"`
	VHistory24hUSD               float64 `json:"vHistory24hUSD"`
	V24hChangePercent            float64 `json:"v24hChangePercent"`
	VBuy24h                      float64 `json:"vBuy24h"`
	VBuy24hUSD                   float64 `json:"vBuy24hUSD"`
	VBuyHistory24h               float64 `json:"vBuyHistory24h"`
	VBuyHistory24hUSD            float64 `json:"vBuyHistory24hUSD"`
	VBuy24hChangePercent         float64 `json:"vBuy24hChangePercent"`
	VSell24h                     float64 `json:"vSell24h"`
	VSell24hUSD                  float64 `json:"vSell24hUSD"`
	VSellHistory24h              float64 `json:"vSellHistory24h"`
	VSellHistory24hUSD           float64 `json:"vSellHistory24hUSD"`
	VSell24hChangePercent        float64 `json:"vSell24hChangePercent"`
	NumberMarkets                int     `json:"numberMarkets"`
}

func (c *Client) GetTokenOverview(ctx context.Context, tokenAddress string, frames string) (*TokenOverview, error) {
	cacheKey := tokenAddress + "|" + frames
	if cachedOverview, ok := c.tokenOverviewCache.Get(cacheKey); ok {
		return cachedOverview, nil
	}
	url := fmt.Sprintf("https://public-api.birdeye.so/defi/token_overview?address=%s&frames=%s", tokenAddress, frames)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-api-key", c.token)
	req.Header.Set("x-chain", "solana")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var result struct {
		Data TokenOverview `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	c.tokenOverviewCache.Set(cacheKey, &result.Data)

	return &result.Data, nil
}
