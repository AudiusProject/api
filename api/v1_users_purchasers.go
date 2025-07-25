package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersPurchasersQueryParams struct {
	Limit       int    `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset      int    `query:"offset" default:"0" validate:"min=0"`
	ContentType string `query:"content_type" validate:"omitempty,oneof=track album playlist"`
	ContentID   int    `query:"content_id" validate:"omitempty,min=1"`
}

func (app *ApiServer) v1UserPurchasers(c *fiber.Ctx) error {
	params := GetUsersPurchasersQueryParams{}
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
		SELECT DISTINCT
			buyer_user_id
		FROM
			usdc_purchases
		WHERE
			` + strings.Join(filters, " AND ") + `
		ORDER BY
			buyer_user_id ASC
		OFFSET @offset
		LIMIT @limit
	;`

	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"userId":      app.getUserId(c),
		"contentType": params.ContentType,
		"contentId":   params.ContentID,
		"offset":      params.Offset,
		"limit":       params.Limit,
	})
}
