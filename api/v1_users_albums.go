package api

import (
	"fmt"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersAlbumsParams struct {
	Limit         int    `query:"limit" default:"20" validate:"min=1,max=100"`
	Offset        int    `query:"offset" default:"0" validate:"min=0"`
	Sort          string `query:"sort" default:"recent" validate:"oneof=recent popular"`
	SortMethod    string `query:"sort_method" default:"" validate:"omitempty,oneof=recent popular"`
	FilterAlbums  string `query:"filter_albums" default:"all" validate:"oneof=all public private"`
	SortDirection string `query:"sort_direction" default:"desc" validate:"oneof=asc desc"`
}

func (app *ApiServer) v1UserAlbums(c *fiber.Ctx) error {
	params := GetUsersAlbumsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)

	sortDir := "DESC"
	if params.SortDirection == "asc" {
		sortDir = "ASC"
	}

	orderClause := fmt.Sprintf("p.created_at %s, p.playlist_id", sortDir)
	if params.SortMethod != "" {
		switch params.SortMethod {
		case "recent":
			orderClause = fmt.Sprintf("p.created_at %s, p.playlist_id", sortDir)
		case "popular":
			orderClause = fmt.Sprintf("(COALESCE(aggregate_playlist.repost_count, 0) + COALESCE(aggregate_playlist.save_count, 0)) %s, p.playlist_id", sortDir)
		}
	}
	userId := app.getUserId(c)

	albumFilter := "(p.is_private = false OR p.playlist_owner_id = @my_id)"
	switch params.FilterAlbums {
	case "public":
		albumFilter = "p.is_private = false"
	case "private":
		albumFilter = "p.is_private = true"
	}

	sql := `
	SELECT p.playlist_id
	FROM playlists p
	JOIN users u ON p.playlist_owner_id = u.user_id
	LEFT JOIN aggregate_playlist ON p.playlist_id = aggregate_playlist.playlist_id
	WHERE p.playlist_owner_id = @user_id
	  AND u.is_deactivated = false
	  AND p.is_delete = false
	  AND p.is_current = true
	  AND p.is_album = true
	  AND ` + albumFilter + `
	ORDER BY ` + orderClause + `
	LIMIT @limit
	OFFSET @offset
	`

	args := pgx.NamedArgs{
		"user_id": userId,
		"my_id":   myId,
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

	albums, err := app.queries.FullPlaylists(c.Context(), dbv1.FullPlaylistsParams{
		GetPlaylistsParams: dbv1.GetPlaylistsParams{
			Ids:  ids,
			MyID: myId,
		},
		OmitTracks: true,
	})
	if err != nil {
		return err
	}

	return c.JSON(fiber.Map{
		"data": albums,
	})
}
