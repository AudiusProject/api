package api

import (
	"time"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

// TODO: Can we get followee_user_id values into this struct?
type GetUsersFeedParams struct {
	Limit  int    `query:"limit" default:"50" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0,max=200"`
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
	followeeIds := queryMulti(c, "followee_user_id")

	var params = GetUsersFeedParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	// follow_set is MATERIALIZED + each branch is a LATERAL with an OFFSET 0
	// optimization fence. Without this, Postgres mis-estimates follow_set
	// cardinality for some users (stale n_distinct stats on
	// follows.follower_user_id) and flips from a sane nested-loop plan to
	// "materialize all 2M reposts of the past year, then merge-join,"
	// turning a sub-second feed into a 9-18 second feed.
	//
	// Per-followee LIMIT 100 (50 for playlists) caps the cost when a
	// followee is very active. The outer query takes only the top-@limit
	// by created_at, so any reposts/tracks past the per-followee top-100
	// can never reach the response anyway.
	sql := `
	WITH
	follow_set AS MATERIALIZED (
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

		-- Track-type reposts.
		(
			SELECT
				'track' as entity_type,
				r.repost_item_id as entity_id,
				min(r.created_at) as created_at
			FROM follow_set fs
			CROSS JOIN LATERAL (
				SELECT repost_item_id, created_at
				FROM reposts
				WHERE reposts.user_id = fs.user_id
					AND reposts.repost_type = 'track'
					AND reposts.created_at < @before
					AND reposts.created_at >= @before - INTERVAL '1 YEAR'
					AND reposts.is_delete = false
				ORDER BY created_at DESC
				LIMIT 100
				OFFSET 0
			) r
			JOIN tracks ON r.repost_item_id = tracks.track_id
				AND tracks.is_delete = false
				AND tracks.is_unlisted = false
				AND tracks.is_available = true
			WHERE @filter in ('all', 'repost')
			GROUP BY entity_id
		)

		UNION ALL

		-- Playlist/album-type reposts.
		(
			SELECT
				r.repost_type::text as entity_type,
				r.repost_item_id as entity_id,
				min(r.created_at) as created_at
			FROM follow_set fs
			CROSS JOIN LATERAL (
				SELECT repost_type, repost_item_id, created_at
				FROM reposts
				WHERE reposts.user_id = fs.user_id
					AND reposts.repost_type <> 'track'
					AND reposts.created_at < @before
					AND reposts.created_at >= @before - INTERVAL '1 YEAR'
					AND reposts.is_delete = false
				ORDER BY created_at DESC
				LIMIT 100
				OFFSET 0
			) r
			JOIN playlists ON r.repost_item_id = playlists.playlist_id
				AND playlists.is_delete = false
				AND playlists.is_private = false
			WHERE @filter in ('all', 'repost')
			GROUP BY entity_type, entity_id
		)

		UNION ALL

		(
			SELECT
				'track' as entity_type,
				t.track_id as entity_id,
				t.created_at
			FROM follow_set fs
			CROSS JOIN LATERAL (
				SELECT track_id, created_at
				FROM tracks
				WHERE owner_id = fs.user_id
					AND created_at < @before
					AND created_at >= @before::timestamp - INTERVAL '1 YEAR'
					AND is_unlisted = false
					AND is_delete = false
					AND stem_of IS NULL
					AND (access_authorities IS NULL
					  OR (COALESCE(@authed_wallet, '') <> ''
					      AND EXISTS (SELECT 1 FROM unnest(access_authorities) aa WHERE lower(aa) = lower(@authed_wallet))))
				ORDER BY created_at DESC
				LIMIT 100
				OFFSET 0
			) t
			WHERE @filter in ('all', 'original')
		)

		UNION ALL

		(
			SELECT
				'playlist' as entity_type,
				p.playlist_id as entity_id,
				p.created_at
			FROM follow_set fs
			CROSS JOIN LATERAL (
				SELECT playlist_id, created_at
				FROM playlists
				WHERE playlist_owner_id = fs.user_id
					AND created_at < @before
					AND created_at >= @before - INTERVAL '1 YEAR'
					AND is_delete = false
					AND is_private = false
				ORDER BY created_at DESC
				LIMIT 50
				OFFSET 0
			) p
			WHERE @filter in ('all', 'original')
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
		"userId":        app.getUserId(c),
		"before":        time.Now(),
		"limit":         params.Limit,
		"offset":        params.Offset,
		"filter":        params.Filter, // original, repost
		"followeeIds":   followeeIds,
		"authed_wallet": app.tryGetAuthedWallet(c),
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
		TrackIds:     trackIds,
		PlaylistIds:  playlistIds,
		MyID:         myId,
		AuthedWallet: app.tryGetAuthedWallet(c),
	})
	if err != nil {
		return err
	}

	// attach entities and filter out items where entity was not found
	filteredStubs := []FeedItem{}
	for _, stub := range stubs {
		if stub.EntityType == "track" {
			if track, ok := loaded.TrackMap[stub.EntityId]; ok {
				stub.Item = track
				filteredStubs = append(filteredStubs, stub)
			}
		} else {
			if playlist, ok := loaded.PlaylistMap[stub.EntityId]; ok {
				stub.Item = playlist
				filteredStubs = append(filteredStubs, stub)
			}
		}
	}

	return c.JSON(fiber.Map{
		"data": filteredStubs,
	})
}
