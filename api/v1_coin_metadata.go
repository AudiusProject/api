package api

import (
	"errors"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// CoinMetadata is the Metaplex Token Metadata off-chain JSON standard.
// This is the document the DBC pool's on-chain `uri` points at for coins
// launched after the Irys -> Audius migration, so the field names here are
// dictated by the standard rather than by our own API conventions and the
// response is served unwrapped (no `data` envelope).
type CoinMetadata struct {
	Name        string `json:"name"`
	Symbol      string `json:"symbol"`
	Description string `json:"description"`
	Image       string `json:"image"`
	ExternalUrl string `json:"external_url"`
	Attributes  []any  `json:"attributes"`
}

func (app *ApiServer) v1CoinMetadata(c *fiber.Ctx) error {
	mint := c.Params("mint")
	if mint == "" {
		return fiber.NewError(fiber.StatusBadRequest, "mint parameter is required")
	}

	var name, ticker string
	var description, logoUri, handle *string
	err := app.pool.QueryRow(c.Context(), `
		SELECT artist_coins.name,
		       artist_coins.ticker,
		       artist_coins.description,
		       artist_coins.logo_uri,
		       users.handle
		FROM artist_coins
		LEFT JOIN users ON users.user_id = artist_coins.user_id
		WHERE artist_coins.mint = $1
	`, mint).Scan(&name, &ticker, &description, &logoUri, &handle)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return fiber.NewError(fiber.StatusNotFound, "Coin not found")
		}
		return err
	}

	appUrl := app.audiusAppUrl
	if appUrl == "" {
		appUrl = "https://audius.co"
	}

	metadata := CoinMetadata{
		Name:        name,
		Symbol:      ticker,
		ExternalUrl: appUrl + "/coins/" + ticker,
		Attributes:  []any{},
	}
	if description != nil && *description != "" {
		metadata.Description = *description
	} else if handle != nil {
		// The launchpad never persists a description - the Fan Club page builds
		// this same sentence client-side, and storing it would make the page cite
		// itself. Regenerate it here so wallets and explorers still get one.
		metadata.Description = defaultCoinDescription(*handle, ticker, appUrl)
	}
	if logoUri != nil {
		metadata.Image = *logoUri
	}

	return c.JSON(metadata)
}

// Kept in step with LAUNCHPAD_COIN_DESCRIPTION in the web client
// (packages/web/src/pages/fan-clubs-launchpad-page/constants.ts).
func defaultCoinDescription(handle string, ticker string, appUrl string) string {
	upper := strings.ToUpper(ticker)
	return fmt.Sprintf(
		"$%s is an artist coin created by @%s on Audius. Learn more at %s/coins/%s",
		upper, handle, appUrl, upper,
	)
}
