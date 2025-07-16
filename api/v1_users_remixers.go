package api

import (
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersRemixersQueryParams struct {
	Limit   int `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset  int `query:"offset" default:"0" validate:"min=0"`
	TrackID int `query:"track_id" validate:"omitempty,min=1"`
}

func (app *ApiServer) v1UsersRemixers(c *fiber.Ctx) error {
	params := GetUsersRemixersQueryParams{}
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
		SELECT DISTINCT
			t2.owner_id
		FROM
			remixes r
		JOIN tracks t1 ON r.parent_track_id = t1.track_id
		JOIN tracks t2 ON r.child_track_id = t2.track_id
		WHERE
			` + strings.Join(filters, " AND ") + `
		ORDER BY
			t2.owner_id ASC
		OFFSET @offset
		LIMIT @limit
	;`

	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"userId":  app.getUserId(c),
		"trackId": params.TrackID,
		"offset":  params.Offset,
		"limit":   params.Limit,
	})
}
