package api

import (
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersGenreTopQueryParams struct {
	Genres []string `query:"genre" validation:"required"`
}

func (app *ApiServer) v1UsersGenreTop(c *fiber.Ctx) error {
	query := GetUsersGenreTopQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &query); err != nil {
		return err
	}

	sql := `
		SELECT users.user_id
		FROM users
		JOIN aggregate_user using (user_id)
		WHERE
			is_current = TRUE
			AND track_count > 0
			AND dominant_genre = ANY(@genres)
		ORDER BY follower_count DESC, user_id ASC
		LIMIT @limit
		OFFSET @offset
	;`

	return app.queryFullUsers(c, sql, pgx.NamedArgs{
		"genres": query.Genres,
	})
}
