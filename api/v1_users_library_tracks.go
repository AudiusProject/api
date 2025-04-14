package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

/*
/v1/full/users/aNzoj/library/tracks?limit=50&offset=50&query=&sort_direction=desc&sort_method=added_date&type=all&user_id=aNzoj
*/

func (app *ApiServer) v1UsersLibraryTracks(c *fiber.Ctx) error {
	// sortMethod := c.Query("sort_method", "added_date")
	// sortDirection := c.Query("sort_direction", "desc")

	sql := `
	WITH library_items AS (
		SELECT
			save_item_id as item_id,
			created_at as item_created_at,
			false as is_purchase
		FROM saves
		WHERE save_type = 'track'
			AND user_id = @userId
			AND is_delete = false

		UNION ALL

		SELECT
			repost_item_id as item_id,
			created_at as item_created_at,
			false as is_purchase
		FROM reposts
		WHERE repost_type = 'track'
			AND user_id = @userId
			AND is_delete = false

		UNION ALL

		SELECT
			content_id as item_id,
			created_at as item_created_at,
			true as is_purchase
		FROM usdc_purchases
		WHERE content_type = 'track'
			AND buyer_user_id = @userId
	)
	SELECT
		item_id,
		max(item_created_at) as item_created_at,
		bool_or(is_purchase) as is_purchase
	FROM library_items
	GROUP BY item_id
	ORDER BY item_created_at DESC
	LIMIT @limit
	OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": c.Locals("userId"),
		"limit":  c.Query("limit"),
		"offset": c.Query("offset"),
	})
	if err != nil {
		return err
	}

	type Activity struct {
		ItemID        int32
		ItemCreatedAt time.Time
		IsPurchase    bool

		Item any `db:"-" json:"item"`
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[Activity])
	if err != nil {
		return err
	}

	// get ids
	trackIds := []int32{}
	for _, i := range items {
		trackIds = append(trackIds, i.ItemID)
	}

	// get tracks
	tracks, err := app.queries.FullTracksKeyed(c.Context(), dbv1.GetTracksParams{
		Ids:  trackIds,
		MyID: c.Locals("myId"),
	})

	// attach
	for idx, item := range items {
		item.Item = tracks[item.ItemID]
		items[idx] = item
	}

	return c.JSON(fiber.Map{
		"data": items,
	})
}
