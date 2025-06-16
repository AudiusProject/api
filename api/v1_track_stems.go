package api

import (
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type TrackStem struct {
	Id           trashid.HashId `db:"track_id" json:"id"`
	ParentId     trashid.HashId `db:"parent_track_id" json:"parent_id"`
	Category     string         `db:"category" json:"category"`
	Cid          string         `db:"track_cid" json:"cid"`
	UserId       trashid.HashId `db:"owner_id" json:"user_id"`
	Blocknumber  int            `db:"blocknumber" json:"blocknumber"`
	OrigFilename string         `db:"orig_filename" json:"orig_filename"`
}

func (app *ApiServer) v1TrackStems(c *fiber.Ctx) error {
	sql := `
	SELECT
	  t.track_id,
	  t.stem_of->>'category' AS category,
	  (t.stem_of->>'parent_track_id')::int AS parent_track_id,
	  t.track_cid,
	  t.owner_id,
	  t.blocknumber,
	  t.orig_filename
	FROM tracks t
	JOIN stems s ON s.child_track_id = t.track_id
	JOIN tracks parent ON parent.track_id = s.parent_track_id
	WHERE t.is_current = true
	  AND t.is_delete = false
	  AND s.parent_track_id = @track_id
	  AND parent.is_delete = false
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"track_id": c.Locals("trackId"),
	})
	if err != nil {
		return err
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByName[TrackStem])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": results,
	})
}
