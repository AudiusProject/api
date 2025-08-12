package api

import (
	"strings"

	"bridgerton.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetTracksMostSharedParams struct {
	Limit     int    `query:"limit" default:"10" validate:"min=1,max=100"`
	Offset    int    `query:"offset" default:"0" validate:"min=0"`
	TimeRange string `query:"time_range" default:"week" validate:"oneof=week month allTime"`
}

func (app *ApiServer) v1TracksMostShared(c *fiber.Ctx) error {
	myId := app.getMyId(c)

	var params = GetTracksMostSharedParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	var shareFilters = []string{
		"share_type = 'track'",
	}
	switch params.TimeRange {
	case "week":
		shareFilters = append(shareFilters, "created_at >= now() - interval '1 week'")
	case "month":
		shareFilters = append(shareFilters, "created_at >= now() - interval '1 month'")
	}

	sql := `
	    WITH most_shared AS (
			SELECT share_item_id, count(*) as share_count
			FROM shares
			WHERE ` + strings.Join(shareFilters, " AND ") + `
			GROUP BY share_item_id
			ORDER BY share_count DESC
		)
		SELECT t.track_id
		FROM most_shared ms
		JOIN tracks t ON ms.share_item_id = t.track_id
		WHERE
			t.is_current = true AND
			t.is_unlisted = false AND
			t.is_available = true AND
			t.is_delete = false
		LIMIT @limit
		OFFSET @offset;
		`
	args := pgx.NamedArgs{}
	args["limit"] = params.Limit
	args["offset"] = params.Offset

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	trackIds, err := pgx.CollectRows(rows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	tracks, err := app.queries.FullTracks(c.Context(), dbv1.FullTracksParams{
		GetTracksParams: dbv1.GetTracksParams{
			Ids:  trackIds,
			MyID: myId,
		},
	})

	if err != nil {
		return err
	}

	return v1TracksResponse(c, tracks)
}
