package api

import (
	"strconv"
	"strings"
	"time"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUserPurchasesQueryParams struct {
	SellerUserID  trashid.HashId   `query:"seller_user_id"`
	ContentIDs    []trashid.HashId `query:"content_ids"`
	ContentType   string           `query:"content_type" validate:"omitempty,oneof=track album playlist"`
	Limit         int              `query:"limit" default:"50" validate:"min=1,max=100"`
	Offset        int              `query:"offset" default:"0" validate:"min=0"`
	SortMethod    string           `query:"sort_method" default:"date" validate:"oneof=content_title artist_name buyer_name date"`
	SortDirection string           `query:"sort_direction" default:"desc" validate:"oneof=asc desc"`
}

type Amount int

func (a Amount) MarshalJSON() ([]byte, error) {
	return []byte(`"` + strconv.Itoa(int(a)) + `"`), nil
}

type Split struct {
	UserID       *int   `db:"user_id" json:"user_id"`
	PayoutWallet string `db:"payout_wallet" json:"payout_wallet"`
	Amount       Amount `db:"amount" json:"amount"`
}

type UsdcPurchase struct {
	SellerUserID trashid.HashId `db:"seller_user_id" json:"seller_user_id"`
	BuyerUserID  trashid.HashId `db:"buyer_user_id" json:"buyer_user_id"`
	Amount       string         `db:"amount" json:"amount"`
	ContentType  string         `db:"content_type" json:"content_type"`
	ContentID    trashid.HashId `db:"content_id" json:"content_id"`
	CreatedAt    time.Time      `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at" json:"updated_at"`
	ExtraAmount  string         `db:"extra_amount" json:"extra_amount"`
	Access       string         `db:"access" json:"access"`
	Slot         int            `db:"slot" json:"slot"`
	Signature    string         `db:"signature" json:"signature"`
	Splits       []Split        `db:"splits" json:"splits"`
}

func (app *ApiServer) v1UsersPurchases(c *fiber.Ctx) error {
	userId := app.getUserId(c)
	params := GetUserPurchasesQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	sortDirection := "DESC"
	if params.SortDirection == "asc" {
		sortDirection = "ASC"
	}

	orderBy := "purchases_with_content.created_at " + sortDirection
	switch params.SortMethod {
	case "artist_name":
		orderBy = "artists.name " + sortDirection + ", " + orderBy
	case "content_title":
		orderBy = "purchases_with_content.content_title " + sortDirection + ", " + orderBy
	case "buyer_name":
		orderBy = "buyers.name " + sortDirection + ", " + orderBy
	}

	filters := []string{"buyer_user_id = @buyerUserId"}

	if params.SellerUserID != 0 {
		filters = append(filters, "seller_user_id = @sellerUserId")
	}

	if len(params.ContentIDs) > 0 {
		filters = append(filters, "content_id = ANY(@contentIds)")
	}

	switch params.ContentType {
	case "track":
		filters = append(filters, "content_type = 'track'")
	case "album":
		filters = append(filters, "content_type = 'album'")
	case "playlist":
		filters = append(filters, "content_type = 'playlist'")
	}

	sql := `
		WITH purchases AS (
			SELECT * FROM usdc_purchases
			WHERE ` + strings.Join(filters, " AND ") + `
		),
		purchases_with_content AS (
			-- Playlists
			SELECT purchases.*,
				playlists.playlist_name AS content_title,
				playlists.playlist_owner_id AS owner_id
			FROM purchases
			JOIN playlists ON playlists.playlist_id = purchases.content_id
			WHERE (content_type = 'playlist' OR content_type = 'album')
				AND (@contentType = '' OR @contentType != 'track')
			-- Tracks
			UNION ALL (
				SELECT purchases.*,
					tracks.title AS content_title,
					tracks.owner_id AS owner_id
				FROM purchases
				JOIN tracks ON tracks.track_id = purchases.content_id
				WHERE content_type = 'track'
					AND (@contentType = '' OR @contentType = 'track')
			)
		)
		SELECT 
			purchases_with_content.seller_user_id,
			purchases_with_content.buyer_user_id,
			purchases_with_content.amount,
			purchases_with_content.content_type,
			purchases_with_content.content_id,
			purchases_with_content.created_at,
			purchases_with_content.updated_at,
			purchases_with_content.extra_amount,
			purchases_with_content.access,
			purchases_with_content.slot,
			purchases_with_content.signature,
			purchases_with_content.splits
		FROM purchases_with_content
		JOIN users AS artists ON artists.user_id = owner_id
		JOIN users AS buyers ON buyers.user_id = buyer_user_id
		ORDER BY ` + orderBy + `
		LIMIT @limit
		OFFSET @offset
	;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"buyerUserId":  userId,
		"sellerUserId": int32(params.SellerUserID),
		"contentIds":   params.ContentIDs,
		"contentType":  params.ContentType,
		"limit":        params.Limit,
		"offset":       params.Offset,
	})
	if err != nil {
		return err
	}

	purchases, err := pgx.CollectRows(rows, pgx.RowToStructByName[UsdcPurchase])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": purchases,
	})
}
