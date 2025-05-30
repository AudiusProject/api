package api

import (
	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersTracksParams struct {
	Limit  int    `query:"limit" default:"20" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0"`
	Sort   string `query:"sort" default:"date"`
	// TODO: Investigate which query param is correct here
	// SortMethod string `query:"sort_method" default:"added_date" validate:"oneof=added_date plays reposts saves title artist_name"`
	SortDirection string `query:"sort_direction" default:"desc" validate:"oneof=asc desc"`
}

func (app *ApiServer) v1UserTracks(c *fiber.Ctx) error {
	params := GetUsersTracksParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)

	sortDir := "DESC"
	if params.SortDirection == "asc" {
		sortDir = "ASC"
	}

	sortField := "coalesce(t.release_date, t.created_at)"
	switch params.Sort {
	case "reposts":
		sortField = "repost_count"
	case "saves":
		sortField = "save_count"
	}

	sql := `
	SELECT track_id
	FROM tracks t
	JOIN users u ON owner_id = u.user_id
	LEFT JOIN aggregate_track USING (track_id)
	WHERE u.handle_lc = LOWER(@handle)
	  AND u.is_deactivated = false
	  AND t.is_delete = false
	  AND t.is_available = true
	  AND (
			t.is_unlisted = false
			OR t.owner_id = @my_id
		)
	  AND t.stem_of is null
	ORDER BY ` + sortField + ` ` + sortDir + `
	LIMIT @limit
	OFFSET @offset
	`

	args := pgx.NamedArgs{
		"handle": c.Params("handle"),
		"my_id":  myId,
	}
	args["limit"] = params.Limit
	args["offset"] = params.Offset

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
