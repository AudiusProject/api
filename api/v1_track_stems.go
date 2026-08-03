package api

import (
	"api.audius.co/trashid"
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
	// Every text/int column here must tolerate NULL: rows written by the Go
	// ETL (post the July 2026 Python cutover) can lack orig_filename, and a
	// stem_of wiped by an explicit-null client update (see
	// OpenAudio/go-openaudio#410 for the same class of bug on CIDs) leaves
	// category/parent_track_id NULL while the stems join row survives. A
	// single NULL scanned into a non-pointer field fails the whole request,
	// which renders every stem on the parent invisible. parent_track_id
	// therefore comes from the stems join key, which is never NULL.
	sql := `
	SELECT
	  t.track_id,
	  COALESCE(t.stem_of->>'category', '') AS category,
	  s.parent_track_id,
	  COALESCE(t.track_cid, '') AS track_cid,
	  t.owner_id,
	  t.blocknumber,
	  COALESCE(t.orig_filename, t.title, '') AS orig_filename
	FROM tracks t
	JOIN stems s ON s.child_track_id = t.track_id
	JOIN tracks parent ON parent.track_id = s.parent_track_id
	WHERE t.is_current = true
	  AND t.is_delete = false
	  AND s.parent_track_id = @track_id
	  AND parent.is_delete = false
	ORDER BY t.track_id
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
