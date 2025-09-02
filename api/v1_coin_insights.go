package api

import (
	"bridgerton.audius.co/api/birdeye"
	"github.com/gagliardetto/solana-go"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type ArtistCoinInsights struct {
	birdeye.TokenOverview
	MembersStatsRow
	DynamicBondingCurve DynamicBondingCurveInsights `json:"dynamic_bonding_curve,omitempty"`
}

type MembersStatsRow struct {
	DbcPool string `db:"dbc_pool" json:"-"`
	Members int    `json:"members"`
}

type DynamicBondingCurveInsights struct {
	Pool          string  `json:"pool"`
	PriceUSD      float64 `json:"price_usd"`
	CurveProgress float64 `json:"curve_progress"`
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
			dbc_pool,
			COUNT(DISTINCT sol_user_balances.user_id) AS members
		FROM artist_coins
		LEFT JOIN sol_user_balances 
			ON sol_user_balances.mint = artist_coins.mint
			AND sol_user_balances.balance > 0
		WHERE artist_coins.mint = @mint
		GROUP BY dbc_pool
		LIMIT 1
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"mint": mint,
	})
	if err != nil {
		return err
	}

	membersStatsRows, err := pgx.CollectExactlyOneRow(rows, pgx.RowToStructByName[MembersStatsRow])
	if err != nil {
		return err
	}

	overview, err := app.birdeyeClient.GetTokenOverview(c.Context(), mint, "24h")
	if err != nil {
		return err
	}
	if overview == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Token overview not found",
		})
	}

	insights := ArtistCoinInsights{
		MembersStatsRow: membersStatsRows,
		TokenOverview:   *overview,
	}

	dbcPool := solana.MustPublicKeyFromBase58(insights.DbcPool)

	dbcPrice, err := app.meteoraDbcClient.GetQuotePrice(
		c.Context(),
		dbcPool,
		overview.Decimals,
		8, // Audio has 8 decimals
	)
	if err != nil {
		return err
	}

	dbcProgress, err := app.meteoraDbcClient.GetPoolCurveProgress(
		c.Context(),
		dbcPool,
	)
	if err != nil {
		return err
	}

	prices, err := app.birdeyeClient.GetPrices(
		c.Context(),
		[]string{app.solanaConfig.MintAudio.String()},
	)
	if err != nil {
		return err
	}
	audioPrice := prices[app.solanaConfig.MintAudio.String()].Value

	insights.DynamicBondingCurve = DynamicBondingCurveInsights{
		Pool:          insights.DbcPool,
		PriceUSD:      dbcPrice * audioPrice,
		CurveProgress: dbcProgress,
	}

	return c.JSON(fiber.Map{
		"data": insights,
	})
}
