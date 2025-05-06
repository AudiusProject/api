package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

/*
/v1/full/users/aNzoj/library/tracks?limit=50&offset=50&query=&sort_direction=desc&sort_method=added_date&type=all&user_id=aNzoj

/v1/full/users/aNzoj/library/tracks?limit=50&offset=0&query=&sort_direction=desc&sort_method=added_date&type=favorite&user_id=aNzoj
*/

func (app *ApiServer) v1UsersLibraryTracks(c *fiber.Ctx) error {
	myId := app.getMyId(c)

	sortField := "item_created_at"
	switch c.Query("sort_method") {
	case "plays":
		sortField = "aggregate_plays.count"
	case "reposts":
		sortField = "aggregate_track.repost_count"
	case "saves":
		sortField = "aggregate_track.save_count"
	case "title":
		sortField = "tracks.title"
	case "artist_name":
		sortField = "users.name"

	}

	sortDirection := "DESC"
	if c.Query("sort_direction") == "asc" {
		sortDirection = "ASC"
	}

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
			AND @actionType in ('favorite', 'all')

		UNION ALL

		SELECT
			repost_item_id as item_id,
			created_at as item_created_at,
			false as is_purchase
		FROM reposts
		WHERE repost_type = 'track'
			AND user_id = @userId
			AND is_delete = false
			AND @actionType in ('repost', 'all')

		UNION ALL

		SELECT
			content_id as item_id,
			created_at as item_created_at,
			true as is_purchase
		FROM usdc_purchases
		WHERE content_type = 'track'
			AND buyer_user_id = @userId
			AND @actionType in ('purchase', 'all')
	),
	deduped as (
		SELECT
			item_id,
			max(item_created_at) as item_created_at,
			bool_or(is_purchase) as is_purchase
		FROM library_items
		JOIN tracks ON track_id = item_id
		WHERE is_unlisted = false OR is_purchase = true
		GROUP BY item_id
	)
	SELECT
		'track_activity_full' as class,
		item_id,
		item_created_at,
		is_purchase
	FROM deduped
	JOIN tracks ON track_id = item_id
	JOIN users ON owner_id = user_id
	LEFT JOIN aggregate_plays ON track_id = play_item_id
	LEFT JOIN aggregate_track USING (track_id)
	WHERE is_unlisted = false OR is_purchase = true
	ORDER BY ` + sortField + ` ` + sortDirection + `
	LIMIT @limit
	OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId":     app.getUserId(c),
		"limit":      c.Query("limit", "50"),
		"offset":     c.Query("offset", "0"),
		"actionType": c.Query("type", "all"),
		// todo: support search / query param
	})
	if err != nil {
		return err
	}

	type Activity struct {
		Class         string    `json:"class"`
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
	trackIds := []int32{}
	for _, i := range items {
		trackIds = append(trackIds, i.ItemID)
	}

	// get tracks
	tracks, err := app.queries.FullTracksKeyed(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:  trackIds,
			MyID: myId,
		},
	})

	// attach
	for idx, item := range items {
		if t, ok := tracks[item.ItemID]; ok {
			item.Item = t
			items[idx] = item
		}
	}

	return c.JSON(fiber.Map{
		"data": items,
	})
}
