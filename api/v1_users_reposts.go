package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersRepostsParams struct {
	Limit  int `query:"limit" default:"100" validate:"min=1,max=100"`
	Offset int `query:"offset" default:"0" validate:"min=0,max=500"`
}

func (app *ApiServer) v1UsersReposts(c *fiber.Ctx) error {
	params := GetUsersRepostsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}
	myId := app.getMyId(c)
	userId := app.getUserId(c)

	sql := `
	SELECT repost_type, repost_item_id, reposts.created_at
	FROM reposts
	LEFT JOIN tracks
		ON repost_type = 'track'
		AND repost_item_id = track_id
		AND tracks.is_delete = false
		AND tracks.is_unlisted = false
		AND tracks.is_available = true
	LEFT JOIN playlists
		ON repost_type != 'track'
		AND repost_item_id = playlist_id
		AND playlists.is_delete = false
		AND playlists.is_private = false
	WHERE
		user_id = @userId
		AND reposts.is_delete = false
		AND (tracks.track_id IS NOT NULL OR playlists.playlist_id IS NOT NULL)
	ORDER BY reposts.created_at DESC
	LIMIT @limit
	OFFSET @offset
	`

	args := pgx.NamedArgs{
		"userId": userId,
		"limit":  params.Limit,
		"offset": params.Offset,
	}

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	type repostRow struct {
		RepostType   string    `json:"item_type"`
		RepostItemId int32     `json:"-"`
		CreatedAt    time.Time `json:"timestamp"`

		Item any `db:"-" json:"item"`
	}

	reposts, err := pgx.CollectRows(rows, pgx.RowToStructByName[repostRow])
	if err != nil {
		return err
	}

	trackIds := []int32{}
	playlistIds := []int32{}

	for _, r := range reposts {
		if r.RepostType == "track" {
			trackIds = append(trackIds, r.RepostItemId)
		} else {
			playlistIds = append(playlistIds, r.RepostItemId)
		}
	}

	loaded, err := app.queries.Parallel(c.Context(), dbv1.ParallelParams{
		TrackIds:    trackIds,
		PlaylistIds: playlistIds,
		MyID:        myId,
	})
	if err != nil {
		return err
	}

	//
	for idx, r := range reposts {
		if r.RepostType == "track" {
			if t, ok := loaded.TrackMap[r.RepostItemId]; ok {
				r.Item = t
			}
		} else {
			if t, ok := loaded.PlaylistMap[r.RepostItemId]; ok {
				r.Item = t
			}
		}
		reposts[idx] = r
	}

	return c.JSON(fiber.Map{
		"data": reposts,
	})
}
