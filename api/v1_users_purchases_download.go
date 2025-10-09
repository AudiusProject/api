package api

import (
	"encoding/csv"
	"strconv"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type UsdcPurchasesDownloadResponse struct {
	Purchases []UsdcPurchaseForDownload `json:"purchases"`
}

type UsdcPurchaseForDownload struct {
	Title        string    `db:"title" json:"title"`
	Link         string    `db:"link" json:"link"`
	SellerName   string    `db:"seller_name" json:"seller_name"`
	SellerUserID int       `db:"seller_user_id" json:"seller_user_id"`
	CreatedAt    time.Time `db:"created_at" json:"date"`
	SalePrice    float64   `db:"sale_price" json:"sale_price"`
	PaidToArtist float64   `db:"-" json:"paid_to_artist"`
	NetworkFee   float64   `db:"-" json:"network_fee"`
	PayExtra     float64   `db:"pay_extra" json:"pay_extra"`
	Total        float64   `db:"-" json:"total"`

	Splits []Split `db:"splits" json:"-"`
}

func (app *ApiServer) userPurchasesForDownload(c *fiber.Ctx) ([]UsdcPurchaseForDownload, error) {
	userId := app.getUserId(c)
	params := GetUsersSalesDownloadQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return nil, err
	}

	var sellerHandle string
	err := app.pool.QueryRow(c.Context(), "SELECT handle FROM users WHERE user_id = @buyerUserId", pgx.NamedArgs{
		"buyerUserId": userId,
	}).Scan(&sellerHandle)
	if err != nil {
		return nil, err
	}

	sql := `
		WITH purchases AS (
			SELECT * FROM usdc_purchases
			WHERE buyer_user_id = @buyerUserId
		),
		purchases_with_content AS (
			-- Playlists
			SELECT purchases.*,
			    users.handle AS seller_handle,
				users.name AS seller_name,
				playlists.playlist_name AS content_title,
				playlists.playlist_owner_id AS owner_id,
				playlist_routes.slug AS content_slug
			FROM purchases
			JOIN playlists ON playlists.playlist_id = purchases.content_id
			JOIN playlist_routes ON playlist_routes.playlist_id = purchases.content_id
			JOIN users ON users.user_id = purchases.seller_user_id
			WHERE (content_type = 'playlist' OR content_type = 'album')
			-- Tracks
			UNION ALL (
				SELECT purchases.*,
				users.handle AS seller_handle,
				users.name AS seller_name,
				users.user_id AS seller_user_id,
					tracks.title AS content_title,
					tracks.owner_id AS owner_id,
					track_routes.slug AS content_slug,
				FROM purchases
				JOIN tracks ON tracks.track_id = purchases.content_id
				JOIN track_routes ON track_routes.track_id = purchases.content_id
				JOIN users ON users.user_id = purchases.seller_user_id
				WHERE content_type = 'track'
			)
		)
		SELECT
			purchases_with_content.content_title AS title,
			COALESCE(@linkBasePath || '/' || purchases_with_content.seller_handle || '/' || purchases_with_content.content_slug, '') AS link,
			purchases_with_content.created_at,
			purchases_with_content.seller_name AS seller_name,
			purchases_with_content.amount / 1000000 AS sale_price,
			purchases_with_content.extra_amount / 1000000 AS pay_extra,
			purchases_with_content.splits,
			purchases_with_content.seller_user_id AS seller_user_id,
		FROM purchases_with_content
		JOIN users ON users.user_id = purchases_with_content.buyer_user_id
		ORDER BY purchases_with_content.created_at DESC;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"buyerUserId":  userId,
		"linkBasePath": app.audiusAppUrl,
	})
	if err != nil {
		return nil, err
	}

	sales, err := pgx.CollectRows(rows, pgx.RowToStructByName[UsdcPurchaseForDownload])
	if err != nil {
		return nil, err
	}

	for i := range sales {
		paidToArtist := 0.0
		networkFee := 0.0
		sellerUserId := int(sales[i].SellerUserID)
		for _, split := range sales[i].Splits {
			if split.PayoutWallet == app.solanaConfig.StakingBridgeUsdcTokenAccount.String() {
				networkFee = float64(split.Amount) / 1000000
			} else if split.UserID != nil && sellerUserId == *split.UserID {
				paidToArtist = float64(split.Amount) / 1000000
			}
		}
		sales[i].PaidToArtist = paidToArtist
		sales[i].NetworkFee = networkFee
		sales[i].Total = sales[i].SalePrice + sales[i].PayExtra
	}

	return sales, nil
}

func (app *ApiServer) v1UsersPurchasesDownloadJson(c *fiber.Ctx) error {
	purchases, err := app.userPurchasesForDownload(c)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": UsdcPurchasesDownloadResponse{
			Purchases: purchases,
		},
	})
}

func (app *ApiServer) v1UsersPurchasesDownloadCsv(c *fiber.Ctx) error {
	sales, err := app.userPurchasesForDownload(c)
	if err != nil {
		return err
	}

	// Set CSV content type header
	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"sales.csv\"")

	// Create CSV writer
	var csvBuilder strings.Builder
	writer := csv.NewWriter(&csvBuilder)

	// Note: csv headers use spaces instead of underscores
	headers := []string{
		"title",
		"link",
		"artist",
		"date",
		"paid to artist",
		"network_fee",
		"pay extra",
		"total",
	}

	if err := writer.Write(headers); err != nil {
		return err
	}

	// Write data rows
	for _, sale := range sales {
		record := []string{
			sale.Title,
			sale.Link,
			sale.SellerName,
			sale.CreatedAt.Format(time.RFC3339),
			strconv.FormatFloat(sale.PaidToArtist, 'f', 6, 64),
			strconv.FormatFloat(sale.NetworkFee, 'f', 6, 64),
			strconv.FormatFloat(sale.PayExtra, 'f', 6, 64),
			strconv.FormatFloat(sale.Total, 'f', 6, 64),
		}

		if err := writer.Write(record); err != nil {
			return err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return err
	}

	return c.SendString(csvBuilder.String())
}
