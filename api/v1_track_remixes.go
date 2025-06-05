package api

import (
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetTrackRemixesParams struct {
	Limit              int    `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset             int    `query:"offset" default:"0" validate:"min=0"`
	SortMethod         string `query:"sort_method" default:"recent" validate:"oneof=recent likes plays"`
	OnlyContestEntries bool   `query:"only_contest_entries" default:"false"`
	OnlyCosigns        bool   `query:"only_cosigns" default:"false"`
}

func (app *ApiServer) v1TrackRemixes(c *fiber.Ctx) error {
	params := GetTrackRemixesParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	myId := app.getMyId(c)

	trackId, err := trashid.DecodeHashId(c.Params("trackId"))
	// TODO: Better error
	if err != nil {
		return err
	}

	var orderClause string
	switch params.SortMethod {
	case "likes":
		orderClause = "coalesce(at.save_count, 0) desc, t.track_id desc"
	case "plays":
		orderClause = "coalesce(ap.count, 0) desc, t.track_id desc"
	case "recent":
		fallthrough
	default:
		orderClause = "t.created_at desc, t.track_id desc"
	}

	var filters []string = []string{
		"t.is_current = true",
		"t.is_delete = false",
		"t.is_unlisted = false",
	}

	if params.OnlyContestEntries {
		filters = append(filters, "(de.created_at < t.created_at AND t.created_at < de.end_date)")
	}

	if params.OnlyCosigns {
		filters = append(filters, "(s.save_item_id IS NOT NULL OR r.repost_item_id IS NOT NULL)")
	}

	// TODO: Add conditionals for cosign and contest entries

	sql := `
		WITH distinct_events AS (
			SELECT DISTINCT ON (entity_id) *
			FROM events
			WHERE event_type = 'remix_contest'
			AND is_deleted = false
			ORDER BY entity_id, created_at DESC
		),
		remix_tracks AS (
			SELECT t.track_id
			FROM tracks t
			JOIN remixes rm ON rm.child_track_id = t.track_id AND rm.parent_track_id = @track_id
			LEFT JOIN tracks pt ON pt.track_id = rm.parent_track_id  -- gives you pt.owner_id
			LEFT JOIN saves s ON s.save_item_id = t.track_id
				AND s.save_type = 'track'
				AND s.is_current = true
				AND s.is_delete = false
				AND s.user_id = pt.owner_id
			LEFT JOIN reposts r ON r.repost_item_id = t.track_id
				AND r.user_id = pt.owner_id
				AND r.repost_type = 'track'
				AND r.is_current = true
				AND r.is_delete = false
			LEFT JOIN aggregate_track at ON at.track_id = t.track_id
			LEFT JOIN aggregate_plays ap ON ap.play_item_id = t.track_id
			LEFT JOIN distinct_events de ON de.entity_id = pt.track_id
			WHERE ` + strings.Join(filters, " AND ") + `
			ORDER BY ` + orderClause + `
		)
		SELECT track_id, (SELECT COUNT(*) FROM remix_tracks) as total_count
		FROM remix_tracks
		LIMIT @limit
		OFFSET @offset;
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"track_id": trackId,
		"limit":    params.Limit,
		"offset":   params.Offset,
	})

	if err != nil {
		return err
	}

	type TrackWithCount struct {
		TrackID    int32 `db:"track_id"`
		TotalCount int64 `db:"total_count"`
	}

	results, err := pgx.CollectRows(rows, pgx.RowToStructByPos[TrackWithCount])
	if err != nil {
		return err
	}

	ids := make([]int32, len(results))
	for i, result := range results {
		ids[i] = result.TrackID
	}

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:  ids,
			MyID: myId,
		},
	})
	if err != nil {
		return err
	}

	var totalCount int64 = 0
	if len(results) > 0 {
		totalCount = results[0].TotalCount
	}

	return c.JSON(fiber.Map{
		"data": fiber.Map{
			"tracks": tracks,
			"count":  totalCount,
		},
	})
}
