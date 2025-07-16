package api

import (
	"strings"

	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUserSalesQueryParams struct {
	BuyerUserID   trashid.HashId   `query:"buyer_user_id"`
	ContentIDs    []trashid.HashId `query:"content_ids"`
	ContentType   string           `query:"content_type" validate:"omitempty,oneof=track album playlist"`
	Limit         int              `query:"limit" default:"50" validate:"min=1,max=100"`
	Offset        int              `query:"offset" default:"0" validate:"min=0"`
	SortMethod    string           `query:"sort_method" default:"date" validate:"oneof=content_title artist_name buyer_name date"`
	SortDirection string           `query:"sort_direction" default:"desc" validate:"oneof=asc desc"`
}

func (app *ApiServer) v1UsersSales(c *fiber.Ctx) error {
	userId := app.getUserId(c)
	params := GetUserSalesQueryParams{}
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

	filters := []string{"seller_user_id = @sellerUserId"}

	if params.BuyerUserID != 0 {
		filters = append(filters, "buyer_user_id = @buyerUserId")
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
		"buyerUserId":  params.BuyerUserID,
		"sellerUserId": userId,
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
