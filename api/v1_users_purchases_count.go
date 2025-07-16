package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UsersPurchasesCount(c *fiber.Ctx) error {
	userId := app.getUserId(c)
	params := GetUserPurchasesQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
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
		SELECT COUNT(*) FROM usdc_purchases
		WHERE ` + strings.Join(filters, " AND ") + `
	;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"buyerUserId":  userId,
		"sellerUserId": int32(params.SellerUserID),
		"contentIds":   params.ContentIDs,
	})
	if err != nil {
		return err
	}

	count, err := pgx.CollectExactlyOneRow(rows, pgx.RowTo[int])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": count,
	})
}
