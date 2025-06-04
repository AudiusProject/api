package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersLibraryPlaylistsParams struct {
	Limit         int    `query:"limit" default:"50" validate:"min=1,max=100"`
	Offset        int    `query:"offset" default:"0" validate:"min=0"`
	ActionType    string `query:"type" default:"all" validate:"oneof=favorite repost purchase all"`
	SortMethod    string `query:"sort_method" default:"reposts" validate:"oneof=added_date plays reposts saves"`
	SortDirection string `query:"sort_direction" default:"desc" validate:"oneof=asc desc"`
}

func (app *ApiServer) v1UsersLibraryPlaylists(c *fiber.Ctx) error {
	myId := app.getMyId(c)

	playlistType := "playlist"
	if c.Params("playlistType") == "albums" {
		playlistType = "album"
	}

	var params = GetUsersLibraryPlaylistsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	sortField := "item_created_at"
	switch params.SortMethod {
	case "reposts":
		sortField = "aggregate_playlist.repost_count"
	case "saves":
		sortField = "aggregate_playlist.save_count"
	}

	sortDirection := "DESC"
	if params.SortDirection == "asc" {
		sortDirection = "ASC"
	}

	sql := `
	WITH playlist_actions AS (
		-- include "own" playlists
		SELECT
			playlist_id as item_id,
			created_at as item_created_at,
			false as is_purchase
		FROM playlists
		WHERE playlist_owner_id = @userId
			AND is_album = (@playlistType = 'album')
			AND is_delete = false
			AND @actionType in ('favorite', 'all')

		UNION ALL

		SELECT
			save_item_id as item_id,
			created_at as item_created_at,
			false as is_purchase
		FROM saves
		WHERE save_type != 'track'
			AND user_id = @userId
			AND is_delete = false
			AND @actionType in ('favorite', 'all')

		UNION ALL

		SELECT
			repost_item_id as item_id,
			created_at as item_created_at,
			false as is_purchase
		FROM reposts
		WHERE repost_type != 'track'
			AND user_id = @userId
			AND is_delete = false
			AND @actionType in ('repost', 'all')

		UNION ALL

		SELECT
			content_id as item_id,
			created_at as item_created_at,
			true as is_purchase
		FROM usdc_purchases
		WHERE content_type = @playlistType::usdc_purchase_content_type
			AND buyer_user_id = @userId
			AND @actionType in ('purchase', 'all')

	),
	deduped as (
		SELECT
			item_id,
			max(item_created_at) as item_created_at,
			bool_or(is_purchase) as is_purchase
		FROM playlist_actions
		GROUP BY item_id
	)
	SELECT
		'collection_activity_full_without_tracks' as class,
		'playlist' as item_type,
		deduped.*
	FROM deduped
	JOIN playlists ON playlist_id = item_id
	LEFT JOIN aggregate_playlist USING (playlist_id)
	WHERE playlists.is_album = (@playlistType = 'album')
	ORDER BY ` + sortField + ` ` + sortDirection + `, item_id desc
	LIMIT @limit
	OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"playlistType": playlistType,
		"userId":       app.getUserId(c),
		"actionType":   params.ActionType,
		"limit":        params.Limit,
		"offset":       params.Offset,
	})
	if err != nil {
		return err
	}

	type Activity struct {
		Class         string    `json:"class"`
		ItemID        int32     `json:"item_id"`
		ItemCreatedAt time.Time `json:"timestamp"`
		IsPurchase    bool      `json:"-"`
		ItemType      string    `json:"item_type"`
		Item          any       `db:"-" json:"item"`
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
	playlists, err := app.queries.FullPlaylistsKeyed(c.Context(), dbv1.FullPlaylistsParams{
		GetPlaylistsParams: dbv1.GetPlaylistsParams{
			Ids:  ids,
			MyID: myId,
		},
	})

	// attach
	for idx, item := range items {
		if p, ok := playlists[item.ItemID]; ok {
			// todo: python code does: exclude playlists with only hidden tracks and empty playlists

			// python API doesn't attach tracks???
			p.Tracks = nil

			item.Item = p
			items[idx] = item
		}
	}

	return c.JSON(fiber.Map{
		"data": items,
	})
}
