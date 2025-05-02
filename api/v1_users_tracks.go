package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1UserTracks(c *fiber.Ctx) error {
	myId := app.getMyId(c)

	sortDir := "DESC"
	if c.Query("sort_direction") == "asc" {
		sortDir = "ASC"
	}

	sortField := "coalesce(t.release_date, t.created_at)"
	switch c.Query("sort") {
	case "reposts":
		sortField = "repost_count"
	case "saves":
		sortField = "save_count"
	}

	sql := `
	SELECT track_id
	FROM tracks t
	JOIN aggregate_track USING (track_id)
	JOIN users u ON owner_id = u.user_id
	JOIN aggregate_plays ON track_id = play_item_id
	WHERE u.handle_lc = LOWER(@handle)
	  AND u.is_deactivated = false
	  AND t.is_delete = false
	  AND t.is_available = true
	  AND t.is_unlisted = false
	  AND t.stem_of is null
	ORDER BY ` + sortField + ` ` + sortDir + `
	LIMIT @limit
	OFFSET @offset
	`

	args := pgx.NamedArgs{
		"handle": c.Params("handle"),
	}
	args["limit"] = c.Query("limit", "20")
	args["offset"] = c.Query("offset", "0")

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	ids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:  ids,
			MyID: myId,
		},
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": tracks,
	})
}
