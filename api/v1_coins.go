package api

import (
	"sync"
	"time"

	"bridgerton.audius.co/api/birdeye"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"
)

type GetArtistCoinsQueryParams struct {
	Tickers  []string         `query:"ticker"`
	Mints    []string         `query:"mint"`
	OwnerIds []trashid.HashId `query:"owner_id"`
	Limit    int              `query:"limit" default:"50" validate:"min=1,max=100"`
	Offset   int              `query:"offset" default:"0" validate:"min=0"`
}

type ArtistCoin struct {
	Ticker                  string                 `json:"ticker"`
	Mint                    string                 `json:"mint"`
	Decimals                int                    `json:"decimals"`
	OwnerId                 trashid.HashId         `db:"user_id" json:"owner_id"`
	Members                 int                    `json:"members"`
	Members24hChangePercent *float64               `json:"members_24h_change_percent"`
	CreatedAt               time.Time              `json:"created_at"`
	TokenInfo               *birdeye.TokenOverview `db:"-" json:"token_info"`
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

	sql := `
		SELECT 
			artist_coins.ticker,
			artist_coins.mint,
			artist_coins.decimals,
			artist_coins.user_id,
			artist_coins.created_at,
			123 as members, -- Placeholder for members count
			50 as members_24h_change_percent -- Placeholder for 24h change percent
		FROM artist_coins
		WHERE 1=1
			` + mintFilter + `
			` + ownerIdFilter + `
			` + tickerFilter + `
		ORDER BY artist_coins.created_at ASC
		LIMIT @limit
		OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"tickers":   queryParams.Tickers,
		"mints":     queryParams.Mints,
		"owner_ids": queryParams.OwnerIds,
		"limit":     queryParams.Limit,
		"offset":    queryParams.Offset,
	})
	if err != nil {
		return err
	}

	coinRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[ArtistCoin])
	if err != nil {
		return err
	}

	wg := sync.WaitGroup{}
	for i := range coinRows {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			mint := coinRows[i].Mint
			overview, err := app.birdeyeClient.GetTokenOverview(c.Context(), mint, "24h")
			if err != nil {
				app.logger.Error("Error fetching token overview",
					zap.String("mint", mint),
					zap.Error(err))
				return
			}
			coinRows[i].TokenInfo = overview
		}(i)
	}
	wg.Wait()

	return c.JSON(fiber.Map{
		"data": coinRows,
	})
}
