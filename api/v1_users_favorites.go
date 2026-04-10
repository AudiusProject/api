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

	myId := app.getMyId(c)

	sql := `
	SELECT
		save_item_id,
		'SaveType.' || save_type as save_item_type, -- concat in "SaveType" to match sqlalchemy bs
		saves.user_id,
		saves.created_at
	FROM saves
	JOIN tracks ON tracks.track_id = saves.save_item_id
	JOIN users ON users.user_id = tracks.owner_id
	WHERE saves.user_id = @userId
	  AND saves.is_delete = false
		AND saves.is_current = true
		AND save_type = 'track'
		AND tracks.is_delete = false
		AND (tracks.is_available = true OR tracks.owner_id = @myId)
		AND users.is_deactivated = false
	ORDER BY saves.blocknumber, save_item_id desc
	LIMIT @limit
	OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": app.getUserId(c),
		"myId":   myId,
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
