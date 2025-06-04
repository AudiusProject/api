package api

import (
	"bridgerton.audius.co/api/fields"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1PlaylistUpdates(c *fiber.Ctx) error {
	userId := app.getUserId(c)

	sql := `
	-- Get playlists that have been updated since the last time the user saw them
	SELECT
		p.playlist_id,
		p.updated_at,
		ps.seen_at
	FROM
		playlists p
	INNER JOIN
		saves s ON
			s.save_item_id = p.playlist_id AND
			s.is_current AND
			NOT s.is_delete AND
			s.save_type = 'playlist' AND
			s.user_id = @userId
	LEFT JOIN
	playlist_seen ps ON
		ps.is_current AND
		ps.playlist_id = p.playlist_id AND
		ps.user_id = @userId
	WHERE
		p.is_current = true AND
		p.is_delete = false AND
		s.created_at < p.updated_at AND
		(ps.seen_at is NULL OR p.updated_at > ps.seen_at)
	ORDER BY p.playlist_id
	;
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": userId,
	})
	if err != nil {
		return err
	}

	type PlaylistUpdate struct {
		PlaylistId trashid.HashId     `db:"playlist_id" json:"playlist_id"`
		UpdatedAt  fields.TimeUnixMs  `db:"updated_at" json:"updated_at"`
		SeenAt     *fields.TimeUnixMs `db:"seen_at" json:"last_seen_at"`
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[PlaylistUpdate])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"playlist_updates": items,
		},
	})
}
