package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
	"golang.org/x/sync/errgroup"
)

func (app *ApiServer) v1UsersHandleReposts(c *fiber.Ctx, minResponse bool) error {
	myId, _ := trashid.DecodeHashId(c.Query("user_id"))
	userId, err := app.resolveUserHandleToId(c.Params("handle"))
	if err != nil {
		return err
	}

	sql := `
	SELECT repost_type, repost_item_id, created_at
	FROM reposts
	WHERE user_id = @userId
	  AND is_delete = false
	ORDER BY created_at DESC
	LIMIT @limit
	OFFSET @offset
	`

	args := pgx.NamedArgs{
		"userId": userId,
	}
	args["limit"] = c.Query("limit", "200")
	args["offset"] = c.Query("offset", "0")

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

	// populate stubs
	g, ctx := errgroup.WithContext(c.Context())

	var trackMap map[int32]dbv1.FullTrack
	var playlistMap map[int32]dbv1.FullPlaylist
	g.Go(func() error {
		var err error
		trackMap, err = app.queries.FullTracksKeyed(ctx, dbv1.GetTracksParams{
			Ids:  trackIds,
			MyID: myId,
		})
		return err
	})
	g.Go(func() error {
		var err error
		playlistMap, err = app.queries.FullPlaylistsKeyed(ctx, dbv1.GetPlaylistsParams{
			Ids:  playlistIds,
			MyID: myId,
		})
		return err
	})
	err = g.Wait()
	if err != nil {
		return err
	}

	//
	for idx, r := range reposts {
		if r.RepostType == "track" {
			if t, ok := trackMap[r.RepostItemId]; ok {
				r.Item = t
			}
		} else {
			if t, ok := playlistMap[r.RepostItemId]; ok {
				r.Item = t
			}
		}
		reposts[idx] = r
	}

	return c.JSON(fiber.Map{
		"data": reposts,
	})
}
