package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersPurchasersCountQueryParams struct {
	ContentType string `query:"content_type" validate:"omitempty,oneof=track album playlist"`
	ContentID   int    `query:"content_id" validate:"omitempty,min=1"`
}

func (app *ApiServer) v1UserPurchasersCount(c *fiber.Ctx) error {
	params := GetUsersPurchasersCountQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	filters := []string{
		"seller_user_id = @userId",
	}

	if params.ContentType != "" {
		filters = append(filters, "content_type = @contentType")
	}

	if params.ContentID != 0 {
		filters = append(filters, "content_id = @contentId")
	}

	sql := `
		SELECT count(DISTINCT buyer_user_id) as count
		FROM
			v_usdc_purchases
		WHERE
			` + strings.Join(filters, " AND ") + `
	;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId":      app.getUserId(c),
		"contentType": params.ContentType,
		"contentId":   params.ContentID,
	})
	if err != nil {
		return err
	}

	count, err := pgx.CollectOneRow(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{"data": count})
}
