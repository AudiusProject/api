package api

import (
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersSalesAggregateQueryParams struct {
	Limit  int `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

type SalesAggregateData struct {
	ContentID     trashid.HashId `json:"content_id" db:"content_id"`
	ContentType   string         `json:"content_type" db:"content_type"`
	PurchaseCount int            `json:"purchase_count" db:"purchase_count"`
}

func (app *ApiServer) v1UsersSalesAggregate(c *fiber.Ctx) error {
	params := GetUsersSalesAggregateQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	sql := `
		SELECT 
			content_id,
			content_type,
			COUNT(buyer_user_id) as purchase_count
		FROM 
			usdc_purchases 
		WHERE 
			seller_user_id = @userId
		GROUP BY 
			content_id, content_type
		ORDER BY 
			purchase_count DESC
		OFFSET @offset
		LIMIT @limit
	;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": app.getUserId(c),
		"offset": params.Offset,
		"limit":  params.Limit,
	})
	if err != nil {
		return err
	}
	defer rows.Close()

	results := []SalesAggregateData{}
	for rows.Next() {
		var item SalesAggregateData
		var contentId int32

		err := rows.Scan(&contentId, &item.ContentType, &item.PurchaseCount)
		if err != nil {
			return err
		}

		item.ContentID = trashid.HashId(contentId)
		results = append(results, item)
	}

	if err := rows.Err(); err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": results,
	})
}
