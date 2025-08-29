package api

import (
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetBestSellingParams struct {
	Limit  int    `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0"`
	Type   string `query:"type" default:"all" validate:"oneof=all track album"`
}

type BestSellingItem struct {
	ContentID   trashid.HashId `db:"content_id" json:"content_id"`
	ContentType string         `db:"content_type" json:"content_type"`
	Title       string         `db:"title" json:"title"`
	OwnerID     trashid.HashId `db:"owner_id" json:"owner_id"`
}

func (app *ApiServer) v1ExploreBestSelling(c *fiber.Ctx) error {
	var params GetBestSellingParams
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	filters := []string{
		"created_at > NOW() - INTERVAL '6 months'",
		"seller_user_id not in (878588928, 90455359, 612014)",
	}
	switch params.Type {
	case "track":
		filters = append(filters, "content_type = 'track'")
	case "album":
		filters = append(filters, "content_type = 'album'")
	}

	sql := `
		WITH ranked_sales AS (
			SELECT content_id, content_type, COUNT(*) AS sales_count
			FROM usdc_purchases
			WHERE ` + strings.Join(filters, " AND ") + `
			GROUP BY content_id, content_type
		),
		results as (
			SELECT
				rs.content_id,
				rs.content_type,
				rs.sales_count,
				t.title,
				t.owner_id
			FROM ranked_sales rs
			JOIN tracks t ON rs.content_id = t.track_id
            WHERE rs.content_type = 'track'
                AND t.is_delete = false
                AND t.is_current = true
                AND t.is_unlisted = false

			UNION ALL

			SELECT
				rs.content_id,
				rs.content_type,
				rs.sales_count,
				p.playlist_name AS title,
				p.playlist_owner_id AS owner_id
			FROM ranked_sales rs
			JOIN playlists p ON rs.content_id = p.playlist_id
            WHERE rs.content_type = 'album'
                AND p.is_delete = false
                AND p.is_current = true
                AND p.is_private = false
		)
		SELECT
			content_id,
			content_type,
			title,
			owner_id
		FROM results
        ORDER BY sales_count DESC, content_id ASC
		LIMIT @limit
		OFFSET @offset;
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"limit":  params.Limit,
		"offset": params.Offset,
	})
	if err != nil {
		return err
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[BestSellingItem])
	if err != nil {
		return err
	}

	if app.getIsFull(c) {
		// related
		trackIds := []int32{}
		playlistIds := []int32{}
		for _, c := range items {
			if c.ContentType == "track" {
				trackIds = append(trackIds, int32(c.ContentID))
			} else if c.ContentType == "album" {
				playlistIds = append(playlistIds, int32(c.ContentID))
			}
		}
		related, err := app.queries.Parallel(c.Context(), dbv1.ParallelParams{
			PlaylistIds: playlistIds,
			TrackIds:    trackIds,
			MyID:        app.getMyId(c),
		})
		if err != nil {
			return err
		}

		return c.JSON(fiber.Map{
			"data": items,
			"related": fiber.Map{
				"playlists": related.PlaylistList(),
				"tracks":    related.TrackList(),
			},
		})
	}

	return c.JSON(fiber.Map{
		"data": items,
	})
}
