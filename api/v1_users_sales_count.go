package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UsersSalesCount(c *fiber.Ctx) error {
	userId := app.getUserId(c)
	params := GetUserSalesQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	contentTypeFilter := "TRUE"
	switch params.ContentType {
	case "track":
		contentTypeFilter = "content_type = 'track'"
	case "album":
		contentTypeFilter = "content_type = 'album'"
	case "playlist":
		contentTypeFilter = "content_type = 'playlist'"
	}

	sql := `
		SELECT COUNT(*) FROM usdc_purchases
		WHERE (@sellerUserId = 0 OR seller_user_id = @sellerUserId)
			AND (@buyerUserId = 0 OR buyer_user_id = @buyerUserId)
			AND (@contentIds::int[] IS NULL OR content_id = ANY(@contentIds::int[]))
			AND (` + contentTypeFilter + `)
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
