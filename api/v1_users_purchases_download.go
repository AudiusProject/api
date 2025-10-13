package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type UsdcPurchaseForDownload struct {
	Title        string    `db:"title" json:"title" csv:"title"`
	Link         string    `db:"link" json:"link" csv:"link"`
	SellerName   string    `db:"seller_name" json:"seller_name" csv:"artist"`
	SellerUserID int       `db:"seller_user_id" json:"seller_user_id" csv:"-"`
	CreatedAt    time.Time `db:"created_at" json:"date" csv:"date"`
	SalePrice    float64   `db:"sale_price" json:"sale_price" csv:"-"`
	PaidToArtist float64   `db:"-" json:"paid_to_artist" csv:"paid to artist"`
	NetworkFee   float64   `db:"-" json:"network_fee" csv:"network fee"`
	PayExtra     float64   `db:"pay_extra" json:"pay_extra" csv:"pay extra"`
	Total        float64   `db:"-" json:"total" csv:"total"`

	Splits []Split `db:"splits" json:"-" csv:"-"`
}

func (app *ApiServer) userPurchasesForDownload(c *fiber.Ctx) ([]UsdcPurchaseForDownload, error) {
	userId := app.getUserId(c)

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
				tracks.title AS content_title,
				tracks.owner_id AS owner_id,
				track_routes.slug AS content_slug
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
			purchases_with_content.extra_amount / 1000000.0 AS pay_extra,
			purchases_with_content.splits,
			purchases_with_content.seller_user_id AS seller_user_id
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
		"data": purchases,
	})
}

func (app *ApiServer) v1UsersPurchasesDownloadCsv(c *fiber.Ctx) error {
	sales, err := app.userPurchasesForDownload(c)
	if err != nil {
		return err
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"sales.csv\"")

	headers := []string{
		"title",
		"link",
		"artist",
		"date",
		"paid to artist",
		"network fee",
		"pay extra",
		"total",
	}

	csvContent, err := WriteCSVFromStructs(sales, headers)
	if err != nil {
		return err
	}

	return c.SendString(csvContent)
}
