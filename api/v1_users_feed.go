package api

import (
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// TODO: Can we get followee_user_id values into this struct?
type GetUsersFeedParams struct {
	Limit  int    `query:"limit" default:"50" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0"`
	Filter string `query:"filter" default:"all" validate:"oneof=all original repost"`
}

// todo: feed currently hard coded to go back: 1 YEAR
// this is probably fine for now, as ES feed only went back 1 MONTH.
// BUT if we want to support going back further:
// - in a loop... start with @before = now()
// - if len(rows) < limit, expand time range until limit is reached.
// this could also work better if client sent some date hints...
//
// we could also get rid of date range filter... it would make feed slower...
// but maybe it'd be okay?
func (app *ApiServer) v1UsersFeed(c *fiber.Ctx) error {
	myId := app.getMyId(c)
	followeeIds := queryMutli(c, "followee_user_id")

	var params = GetUsersFeedParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	sql := `
	WITH
	follow_set AS (
		SELECT followee_user_id AS user_id
		FROM follows
		WHERE
		follower_user_id = @userId
		AND is_delete = false

		UNION ALL

		-- If the user has specified any followee_user_ids, include them.
		SELECT unnest(@followeeIds::int[]) AS user_id
		WHERE @followeeIds IS NOT NULL
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
				AND reposts.created_at >= @before - INTERVAL '1 YEAR'
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
				AND created_at >= @before::timestamp - INTERVAL '1 YEAR'
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
				AND created_at >= @before - INTERVAL '1 YEAR'
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
		"userId":      app.getUserId(c),
		"before":      time.Now(),
		"limit":       params.Limit,
		"offset":      params.Offset,
		"filter":      params.Filter, // original, repost
		"followeeIds": followeeIds,
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
