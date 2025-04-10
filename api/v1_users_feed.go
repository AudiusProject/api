package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// todo: some dedupe stuff
func (app *ApiServer) v1UsersFeed(c *fiber.Ctx) error {

	sql := `
	WITH
	follow_set AS (
		SELECT followee_user_id AS user_id
		FROM follows
		WHERE
			follower_user_id = @userId
			AND is_delete = false
	),
	history as (

		(
			SELECT
				user_id as actor_id,
				'repost' as verb,
				repost_type as obj_type,
				repost_item_id as obj_id,
				reposts.created_at
			FROM reposts
			JOIN follow_set using (user_id)
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
				reposts.created_at < @before
				AND reposts.created_at >= @before - INTERVAL '30 DAYS'
				AND reposts.is_delete = false
				AND (tracks.track_id IS NOT NULL OR playlists.playlist_id IS NOT NULL)
		)

		UNION ALL

		(
			SELECT
				user_id as actor_id,
				'post' as verb,
				'track' as obj_type,
				track_id as obj_id,
				created_at
			from tracks
			join follow_set on owner_id = user_id
			where created_at < @before
				and created_at >= @before::timestamp - INTERVAL '30 DAYS'
				and is_unlisted = false
				and is_delete = false
				and stem_of is null
		)

		UNION ALL

		(
			SELECT
				user_id as actor_id,
				'post' as verb,
				'playlist' as obj_type,
				playlist_id as obj_id,
				created_at
			from playlists
			join follow_set on playlist_owner_id = user_id
			where created_at < @before
				and created_at >= @before::timestamp - INTERVAL '30 DAYS'
				and is_delete = false
				AND is_private = false
		)

	)
	SELECT * FROM history
	ORDER BY created_at asc
	LIMIT @limit
	OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": c.Locals("userId"),
		"before": time.Now(),
		"limit":  c.Query("limit", "50"),
		"offset": c.Query("offset", "0"),
	})
	if err != nil {
		return err
	}

	type FeedItem struct {
		ActorID   int
		Verb      string
		ObjType   string `json:"type"`
		ObjID     int32
		CreatedAt time.Time

		Item any `db:"-" json:"item"`
	}

	stubs, err := pgx.CollectRows(rows, pgx.RowToStructByName[FeedItem])
	if err != nil {
		return err
	}

	// todo: remove duplicates
	// something like:
	// https://github.com/stereosteve/Elemental/blob/master/server/db/query-feed.ts#L77-L85

	trackIds := []int32{}
	playlistIds := []int32{}
	for _, stub := range stubs {
		if stub.ObjType == "track" {
			trackIds = append(trackIds, stub.ObjID)
		} else {
			playlistIds = append(playlistIds, stub.ObjID)
		}
	}

	loaded, err := app.queries.Parallel(c.Context(), dbv1.ParallelParams{
		TrackIds:    trackIds,
		PlaylistIds: playlistIds,
		MyID:        c.Locals("myId"),
	})
	if err != nil {
		return err
	}

	for idx, stub := range stubs {
		if stub.ObjType == "track" {
			stub.Item = loaded.TrackMap[stub.ObjID]
		} else {
			stub.Item = loaded.PlaylistMap[stub.ObjID]
		}
		stubs[idx] = stub
	}

	return c.JSON(fiber.Map{
		"data": stubs,
	})
}
