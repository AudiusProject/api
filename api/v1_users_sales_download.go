package api

import (
	"fmt"
	"time"

	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersSalesDownloadQueryParams struct {
	GranteeUserID *trashid.HashId `query:"grantee_user_id"`
}

type UsdcSaleWithEmailResponse struct {
	Sales []UsdcSaleWithEmail `json:"sales"`
}

type UsdcSaleWithEmail struct {
	Title          string    `db:"title" json:"title" csv:"title"`
	Link           string    `db:"link" json:"link" csv:"link"`
	PurchasedBy    string    `db:"purchased_by" json:"purchased_by" csv:"purchased by"`
	CreatedAt      time.Time `db:"created_at" json:"date" csv:"date"`
	SalePrice      float64   `db:"sale_price" json:"sale_price" csv:"sale price"`
	NetworkFee     float64   `db:"-" json:"network_fee" csv:"network_fee"`
	PayExtra       float64   `db:"pay_extra" json:"pay_extra" csv:"pay extra"`
	Total          float64   `db:"-" json:"total" csv:"total"`
	Country        string    `db:"country" json:"country" csv:"country"`
	EncryptedEmail *string   `db:"encrypted_email" json:"encrypted_email" csv:"-"`
	EncryptedKey   *string   `db:"encrypted_key" json:"encrypted_key" csv:"-"`
	BuyerUserID    *int      `db:"buyer_user_id" json:"buyer_user_id" csv:"-"`
	IsInitial      *bool     `db:"is_initial" json:"is_initial" csv:"-"`
	PubkeyBase64   *string   `db:"pubkey_base64" json:"pubkey_base64" csv:"-"`

	Splits []Split `db:"splits" json:"-" csv:"-"`
}

func (app *ApiServer) userSalesForDownload(c *fiber.Ctx) ([]UsdcSaleWithEmail, error) {
	userId := app.getUserId(c)
	params := GetUsersSalesDownloadQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return nil, err
	}

	emailAccessJoin := `LEFT OUTER JOIN email_access ON
	  	email_access.email_owner_user_id = purchases_with_content.buyer_user_id
		AND email_access.receiving_user_id = @sellerUserId
		AND email_access.grantor_user_id = purchases_with_content.buyer_user_id`
	if params.GranteeUserID != nil {
		emailAccessJoin = `LEFT OUTER JOIN email_access ON
				email_access.email_owner_user_id = purchases_with_content.buyer_user_id
				AND email_access.receiving_user_id = @granteeUserId
				AND email_access.grantor_user_id = @sellerUserId`
	}

	var sellerHandle string
	err := app.pool.QueryRow(c.Context(), "SELECT handle FROM users WHERE user_id = @sellerUserId", pgx.NamedArgs{
		"sellerUserId": userId,
	}).Scan(&sellerHandle)
	if err != nil {
		return nil, err
	}

	sql := `
		WITH purchases AS (
			SELECT * FROM usdc_purchases
			WHERE seller_user_id = @sellerUserId
		),
		purchases_with_content AS (
			-- Playlists
			SELECT purchases.*,
				playlists.playlist_name AS content_title,
				playlists.playlist_owner_id AS owner_id,
				playlist_routes.slug AS content_slug
			FROM purchases
			JOIN playlists ON playlists.playlist_id = purchases.content_id
			JOIN playlist_routes ON playlist_routes.playlist_id = purchases.content_id
			WHERE (content_type = 'playlist' OR content_type = 'album')
			-- Tracks
			UNION ALL (
				SELECT purchases.*,
					tracks.title AS content_title,
					tracks.owner_id AS owner_id,
					track_routes.slug AS content_slug
				FROM purchases
				JOIN tracks ON tracks.track_id = purchases.content_id
				JOIN track_routes ON track_routes.track_id = purchases.content_id
				WHERE content_type = 'track'
			)
		)
		SELECT
			purchases_with_content.content_title AS title,
			COALESCE(@linkBasePath || purchases_with_content.content_slug, '') AS link,
			purchases_with_content.created_at,
			purchases_with_content.amount / 1000000 AS sale_price,
			purchases_with_content.extra_amount / 1000000 AS pay_extra,
			purchases_with_content.splits,
			COALESCE(purchases_with_content.country, '') AS country,
			COALESCE(users.name, '') AS purchased_by,
			users.user_id AS buyer_user_id,
			encrypted_emails.encrypted_email,
			email_access.encrypted_key,
			email_access.is_initial,
			user_pubkeys.pubkey_base64
		FROM purchases_with_content
		JOIN users ON users.user_id = purchases_with_content.buyer_user_id
		LEFT OUTER JOIN encrypted_emails ON encrypted_emails.email_owner_user_id = purchases_with_content.buyer_user_id
		` + emailAccessJoin + `
		LEFT OUTER JOIN user_pubkeys ON user_pubkeys.user_id = purchases_with_content.buyer_user_id
		ORDER BY purchases_with_content.created_at DESC;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"sellerUserId":  userId,
		"granteeUserId": params.GranteeUserID,
		"linkBasePath":  fmt.Sprintf("%s/%s/", app.audiusAppUrl, sellerHandle),
	})
	if err != nil {
		return nil, err
	}

	sales, err := pgx.CollectRows(rows, pgx.RowToStructByName[UsdcSaleWithEmail])
	if err != nil {
		return nil, err
	}

	for i := range sales {
		networkFee := 0.0
		total := 0.0
		for _, split := range sales[i].Splits {
			if split.PayoutWallet == app.solanaConfig.StakingBridgeUsdcTokenAccount.String() {
				networkFee = 0 - float64(split.Amount)/1000000
			} else if split.UserID != nil && int(userId) == *split.UserID {
				total = float64(split.Amount)/1000000 + sales[i].PayExtra
			}
		}
		sales[i].NetworkFee = networkFee
		sales[i].Total = total
	}

	return sales, nil
}

func (app *ApiServer) v1UsersSalesDownloadJson(c *fiber.Ctx) error {
	sales, err := app.userSalesForDownload(c)
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": UsdcSaleWithEmailResponse{
			Sales: sales,
		},
	})
}

func (app *ApiServer) v1UsersSalesDownloadCsv(c *fiber.Ctx) error {
	sales, err := app.userSalesForDownload(c)
	if err != nil {
		return err
	}

	c.Set("Content-Type", "text/csv")
	c.Set("Content-Disposition", "attachment; filename=\"sales.csv\"")

	csvContent, err := WriteCSVFromStructs(sales, []string{
		"title",
		"link",
		"purchased by",
		"date",
		"sale price",
		"network_fee",
		"pay extra",
		"total",
		"country",
	})
	if err != nil {
		return err
	}

	return c.SendString(csvContent)
}
