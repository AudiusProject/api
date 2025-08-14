package api

import (
	"strings"
	"time"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersHistoryParams struct {
	Limit         int    `query:"limit" default:"100" validate:"min=1,max=100"`
	Offset        int    `query:"offset" default:"0" validate:"min=0"`
	SortMethod    string `query:"sort_method" default:"last_listen_date" validate:"oneof=title artist_name release_date last_listen_date plays reposts saves most_listens_by_user"`
	SortDirection string `query:"sort_direction" default:"desc" validate:"oneof=asc desc"`
	Query         string `query:"query" default:""`
}

func (app *ApiServer) v1UsersHistory(c *fiber.Ctx) error {
	userId := app.getUserId(c)
	myId := app.getMyId(c)

	params := GetUsersHistoryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	sortDirection := "DESC"
	if params.SortDirection == "asc" {
		sortDirection = "ASC"
	}

	sortField := ""
	switch params.SortMethod {
	case "last_listen_date":
		sortField = "history.timestamp"
	case "plays":
		sortField = "aggregate_plays.count"
	case "reposts":
		sortField = "aggregate_track.repost_count"
	case "saves":
		sortField = "aggregate_track.save_count"
	case "title":
		sortField = "tracks.title"
	case "artist_name":
		sortField = "users.name"
	case "release_date":
		sortField = "tracks.release_date"
	case "most_listens_by_user":
		sortField = "history.play_count"
	}

	orderBy := ""
	// Listen history is ordered by timestamp DESC, so no need to sort by it explicitly.
	if sortField != "history.timestamp" || sortDirection != "DESC" {
		orderBy = "ORDER BY " + sortField + " " + sortDirection
	}

	class := "track_activity"
	if app.getIsFull(c) {
		class = "track_activity_full"
	}

	filters := []string{}
	if params.Query != "" {
		filters = append(filters, "tracks.title ILIKE '%' || @query || '%' OR users.name ILIKE '%' || @query || '%'")
	}

	if myId == 0 || myId != userId {
		filters = append(filters, "tracks.is_unlisted = false")
	}

	filterString := ""
	if len(filters) > 0 {
		filterString = "WHERE " + strings.Join(filters, " AND ")
	}

	sql := `
	WITH history AS (
		SELECT
			(jsonb_array_elements(listening_history)->>'track_id')::int AS track_id,
			(jsonb_array_elements(listening_history)->>'play_count')::int AS play_count,
			(jsonb_array_elements(listening_history)->>'timestamp')::timestamp AS timestamp
		FROM user_listening_history
		WHERE user_id = @userId
	)
	SELECT history.track_id AS item_id,
		history.timestamp AS item_created_at,
		'track' AS item_type,
		@class AS class
	FROM history
	LEFT JOIN tracks ON history.track_id = tracks.track_id
	LEFT JOIN aggregate_track ON tracks.track_id = aggregate_track.track_id
	LEFT JOIN users ON tracks.owner_id = users.user_id
	LEFT JOIN aggregate_plays ON history.track_id = aggregate_plays.play_item_id
	` + filterString + `
	` + orderBy + `
	LIMIT @limit OFFSET @offset
	;
	`
	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId": userId,
		"limit":  params.Limit,
		"offset": params.Offset,
		"query":  params.Query,
		"class":  class,
	})
	if err != nil {
		return err
	}

	type Activity struct {
		Class         string    `db:"class" json:"class"`
		ItemID        int32     `db:"item_id" json:"item_id"`
		ItemCreatedAt time.Time `db:"item_created_at" json:"timestamp"`
		ItemType      string    `db:"item_type" json:"item_type"`

		Item any `db:"-" json:"item"`
	}

	items, err := pgx.CollectRows(rows, pgx.RowToStructByName[Activity])
	if err != nil {
		return err
	}

	// get ids
	trackIds := []int32{}
	for _, i := range items {
		trackIds = append(trackIds, i.ItemID)
	}

	// get tracks
	tracks, err := app.queries.FullTracksKeyed(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:             trackIds,
			MyID:            myId,
			IncludeUnlisted: true,
		},
	})
	if err != nil {
		return err
	}

	// attach
	for idx, item := range items {
		if t, ok := tracks[item.ItemID]; ok {
			if app.getIsFull(c) {
				item.Item = t
			} else {
				item.Item = dbv1.ToMinTrack(t)
			}
			items[idx] = item
		}
	}

	return c.JSON(fiber.Map{
		"data": items,
	})
}
