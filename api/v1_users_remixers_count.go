package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersRemixersCountQueryParams struct {
	TrackID int `query:"track_id" validate:"omitempty,min=1"`
}

func (app *ApiServer) v1UsersRemixersCount(c *fiber.Ctx) error {
	params := GetUsersRemixersCountQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	filters := []string{
		"t1.owner_id = @userId",
		"t1.is_delete = FALSE",
		"t1.is_unlisted = FALSE",
		"t2.is_delete = FALSE",
		"t2.is_unlisted = FALSE",
	}

	if params.TrackID != 0 {
		filters = append(filters, "r.parent_track_id = @trackId")
	}

	sql := `
		SELECT count(DISTINCT t2.owner_id) as count
		FROM
			remixes r
		JOIN tracks t1 ON r.parent_track_id = t1.track_id
		JOIN tracks t2 ON r.child_track_id = t2.track_id
		WHERE ` + strings.Join(filters, " AND ") + `
	;`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId":  app.getUserId(c),
		"trackId": params.TrackID,
	})

	if err != nil {
		return err
	}

	count, err := pgx.CollectOneRow(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": count,
	})
}
