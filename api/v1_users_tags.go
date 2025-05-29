package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

/*
/v1/users/7eP5n/tags?limit=5
*/

type GetUsersTagsParams struct {
	Limit int `query:"limit" default:"10" validate:"min=1,max=100"`
}

func (app *ApiServer) v1UsersTags(c *fiber.Ctx) error {
	params := GetUsersTagsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}
	sql := `
	SELECT tag
	FROM (
		SELECT TRIM(regexp_split_to_table(tags, ',')) AS tag
		FROM tracks
		WHERE owner_id = @userId
			AND is_unlisted = false
			AND is_delete = false
			AND stem_of is null
	) AS split_tags
	WHERE tag != ''
	GROUP BY tag
	ORDER BY COUNT(*) DESC
	LIMIT @limit;
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": app.getUserId(c),
		"limit":  params.Limit,
	})
	if err != nil {
		return err
	}

	tags, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": tags,
	})
}
