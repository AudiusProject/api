package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// todo: time range beyond 30 days
// - how + when to expand time range,
// - how to de-dupe items from appearing twice when expanding time range... is tricky
func (app *ApiServer) v1UsersFeed(c *fiber.Ctx) error {
	myId := app.getMyId(c)

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
				repost_type as entity_type,
				repost_item_id as entity_id,
				min(reposts.created_at) as created_at
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
				@filter in ('all', 'repost')
				AND reposts.created_at < @before
				AND reposts.created_at >= @before - INTERVAL '30 DAYS'
				AND reposts.is_delete = false
				AND (tracks.track_id IS NOT NULL OR playlists.playlist_id IS NOT NULL)
			GROUP BY entity_type, entity_id
		)

		UNION ALL

		(
			SELECT
				'track' as entity_type,
				track_id as entity_id,
				created_at
			from tracks
			join follow_set on owner_id = user_id
			where @filter in ('all', 'original')
				AND created_at < @before
				AND created_at >= @before::timestamp - INTERVAL '30 DAYS'
				AND is_unlisted = false
				AND is_delete = false
				AND stem_of is null
		)

		UNION ALL

		(
			SELECT
				'playlist' as entity_type,
				playlist_id as entity_id,
				created_at
			from playlists
			join follow_set on playlist_owner_id = user_id
			where @filter in ('all', 'original')
				AND created_at < @before
				AND created_at >= @before - INTERVAL '30 DAYS'
				AND is_delete = false
				AND is_private = false
		)

	)
	SELECT
		entity_type,
		entity_id,
		max(created_at) as created_at
	FROM history
	GROUP BY entity_type, entity_id
	ORDER BY created_at DESC
	LIMIT @limit
	OFFSET @offset
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": c.Locals("userId"),
		"before": time.Now(),
		// "limit":  c.Query("limit", "50"),
		"limit":  40,
		"offset": c.Query("offset", "0"),
		"filter": c.Query("filter", "all"), // original, repost
	})
	if err != nil {
		return err
	}

	type FeedItem struct {
		EntityType string    `json:"type"`
		EntityId   int32     `json:"-"`
		CreatedAt  time.Time `json:"timestamp"`

		Item any `db:"-" json:"item"`
	}

	stubs, err := pgx.CollectRows(rows, pgx.RowToStructByName[FeedItem])
	if err != nil {
		return err
	}

	// todo: remove loose tracks that appear in playlist?

	trackIds := []int32{}
	playlistIds := []int32{}
	for _, stub := range stubs {
		if stub.EntityType == "track" {
			trackIds = append(trackIds, stub.EntityId)
		} else {
			playlistIds = append(playlistIds, stub.EntityId)
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

	for idx, stub := range stubs {
		if stub.EntityType == "track" {
			stub.Item = loaded.TrackMap[stub.EntityId]
		} else {
			stub.Item = loaded.PlaylistMap[stub.EntityId]
		}
		stubs[idx] = stub
	}

	return c.JSON(fiber.Map{
		"data": stubs,
	})
}
