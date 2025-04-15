package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UsersLibraryPlaylists(c *fiber.Ctx) error {

	itemType := "playlist" // or album
	sortField := "item_created_at"
	sortDirection := "DESC"

	sql := `
	WITH playlist_stuff AS (
		-- include "own" playlists
		SELECT
			playlist_id as item_id,
			created_at as item_created_at,
			false as is_purchase
		FROM playlists
		WHERE playlist_owner_id = @userId
			AND is_album = (@itemType = 'album')
			AND is_delete = false
			AND @verb in ('favorite', 'all')

		UNION ALL

		SELECT
			save_item_id as item_id,
			created_at as item_created_at,
			false as is_purchase
		FROM saves
		WHERE save_type != 'track'
			AND user_id = @userId
			AND is_delete = false
			AND @verb in ('favorite', 'all')

		UNION ALL

		SELECT
			repost_item_id as item_id,
			created_at as item_created_at,
			false as is_purchase
		FROM reposts
		WHERE repost_type != 'track'
			AND user_id = @userId
			AND is_delete = false
			AND @verb in ('repost', 'all')

		UNION ALL

		SELECT
			content_id as item_id,
			created_at as item_created_at,
			true as is_purchase
		FROM usdc_purchases
		WHERE content_type = @itemType::usdc_purchase_content_type
			AND buyer_user_id = @userId
			AND @verb in ('purchase', 'all')

	),
	deduped as (
		SELECT
			item_id,
			max(item_created_at) as item_created_at,
			bool_or(is_purchase) as is_purchase
		FROM playlist_stuff
		GROUP BY item_id
	)
	SELECT deduped.* FROM deduped
	JOIN playlists ON playlist_id = item_id
	WHERE is_album = (@itemType = 'album')
	ORDER BY ` + sortField + ` ` + sortDirection + `, item_id desc
	LIMIT @limit
	OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"itemType": itemType,
		"userId":   c.Locals("userId"),
		"verb":     c.Query("type", "all"),
		"limit":    c.Query("limit", "50"),
		"offset":   c.Query("offset", "0"),
	})
	if err != nil {
		return err
	}

	type Activity struct {
		// Class         string    `json:"class"`
		ItemID        int32     `json:"item_id"`
		ItemCreatedAt time.Time `json:"timestamp"`
		IsPurchase    bool      `json:"-"`

		Item any `db:"-" json:"item"`
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[Activity])
	if err != nil {
		return err
	}

	// get ids
	ids := []int32{}
	for _, i := range items {
		ids = append(ids, i.ItemID)
	}

	// get playlists
	playlists, err := app.queries.FullPlaylistsKeyed(c.Context(), dbv1.GetPlaylistsParams{
		Ids:  ids,
		MyID: app.getMyId(c),
	})

	// attach
	for idx, item := range items {
		if t, ok := playlists[item.ItemID]; ok {
			item.Item = t
			items[idx] = item
		}
	}

	return c.JSON(fiber.Map{
		"data": items,
	})
}
