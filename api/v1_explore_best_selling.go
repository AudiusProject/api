package api

import (
	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetBestSellingParams struct {
	Limit  int `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
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

	sql := `
		WITH ranked_sales AS (
			SELECT content_id, content_type, COUNT(*) AS sales_count
			FROM usdc_purchases
			WHERE created_at > NOW() - INTERVAL '6 months'
			GROUP BY content_id, content_type
			ORDER BY sales_count DESC
		),
		results as (
			SELECT
				rs.content_id,
				rs.content_type,
				rs.sales_count,
				t.title,
				t.owner_id,
				t.is_delete,
				t.is_unlisted,
				t.is_current
			FROM ranked_sales rs
			JOIN tracks t ON rs.content_type = 'track' AND rs.content_id = t.track_id

			UNION ALL

			SELECT
				rs.content_id,
				rs.content_type,
				rs.sales_count,
				a.playlist_name AS title,
				a.playlist_owner_id AS owner_id,
				a.is_delete,
				a.is_private as is_unlisted,
				a.is_current
			FROM ranked_sales rs
			JOIN playlists a ON rs.content_type = 'album' AND rs.content_id = a.playlist_id
		)
		SELECT
			content_id,
			content_type,
			title,
			owner_id
		FROM results
		WHERE is_delete = false
			AND is_unlisted = false
			AND is_current = true
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
