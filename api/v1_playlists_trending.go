package api

import (
	"net/http"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

func (app *ApiServer) v1PlaylistsTrending(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	authedUserId := app.getAuthedUserId(c)
	authedWallet := app.getAuthedWallet(c)

	sql := `
	SELECT
		save_item_id as playlist_id
		-- count(distinct user_id) as save_count,
		-- sum(follower_count) as network_size
	FROM saves
	JOIN aggregate_user USING (user_id)
	WHERE save_type != 'track'
		AND saves.is_delete = false
		AND saves.created_at > NOW() - INTERVAL '7 days'
	GROUP BY playlist_id
	ORDER BY sum(follower_count) DESC
	LIMIT @limit
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"limit": c.Query("limit", "50"),
	})
	if err != nil {
		return err
	}

	ids, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	playlists, err := app.queries.FullPlaylists(c.Context(), dbv1.FullPlaylistsParams{
		GetPlaylistsParams: dbv1.GetPlaylistsParams{
			Ids:  ids,
			MyID: myId,
		},
		AuthedUserId:        authedUserId,
		AuthedWallet:        authedWallet,
		IsAuthorizedRequest: app.isAuthorizedRequest,
	})

	return c.Status(http.StatusOK).JSON(fiber.Map{
		"data": playlists,
	})
}
