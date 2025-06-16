package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UsersSalesCount(c *fiber.Ctx) error {
	userId := app.getUserId(c)
	params := GetUserSalesQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
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
		SELECT COUNT(*) FROM usdc_purchases
		WHERE ` + strings.Join(filters, " AND ") + `
	;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"buyerUserId":  params.BuyerUserID,
		"sellerUserId": userId,
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
