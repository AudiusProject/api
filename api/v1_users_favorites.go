package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUserFavoritesQueryParams struct {
	Limit  int `query:"limit" default:"50" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0"`
}

type Favorite struct {
	FavoriteItemID        int32     `db:"save_item_id" json:"favorite_item_id"`
	FavoriteItemType      string    `db:"save_item_type" json:"favorite_type"`
	UserId                int32     `db:"user_id" json:"user_id"`
	FavoriteItemCreatedAt time.Time `db:"created_at" json:"created_at"`
}

func (app *ApiServer) v1UsersFavorites(c *fiber.Ctx) error {
	params := GetUserFavoritesQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	sql := `
	SELECT
		save_item_id,
		'SaveType.' || save_type as save_item_type, -- concat in "SaveType" to match sqlalchemy bs
		user_id,
		created_at
	FROM saves
	WHERE user_id = @userId
	  AND is_delete = false
		AND is_current = true
		AND save_type = 'track'
	ORDER BY blocknumber, save_item_id desc
	LIMIT @limit
	OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": app.getUserId(c),
		"limit":  params.Limit,
		"offset": params.Offset,
	})
	if err != nil {
		return err
	}

	saves, err := pgx.CollectRows(rows, pgx.RowToStructByName[Favorite])
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": saves,
	})
}
