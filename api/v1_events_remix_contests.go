package api

import (
	"strings"

	"api.audius.co/api/dbv1"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetRemixContestsParams struct {
	Limit  int    `query:"limit" default:"25" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0"`
	Status string `query:"status" default:"all" validate:"oneof=active ended all"`
}

// v1EventsRemixContests returns remix-contest events from the events table,
// ordered with currently-active contests first (by soonest-ending end_date),
// followed by ended contests (most-recently-ended first). Supports pagination
// and an optional `status` filter (active | ended | all).
func (app *ApiServer) v1EventsRemixContests(c *fiber.Ctx) error {
	params := GetRemixContestsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	filters := []string{
		"e.event_type = 'remix_contest'",
		"e.is_deleted = false",
		"(e.entity_type != 'track' OR (t.track_id IS NOT NULL AND t.is_delete = false))",
	}

	switch params.Status {
	case "active":
		filters = append(filters, "(e.end_date IS NULL OR e.end_date > NOW())")
	case "ended":
		filters = append(filters, "(e.end_date IS NOT NULL AND e.end_date <= NOW())")
	}

	sql := `
		SELECT
			e.event_id,
			e.entity_type::event_entity_type AS entity_type,
			e.user_id,
			e.entity_id,
			e.event_type::event_type AS event_type,
			e.end_date,
			e.is_deleted,
			e.created_at,
			e.updated_at,
			e.event_data
		FROM events e
		LEFT JOIN tracks t ON t.track_id = e.entity_id
			AND t.is_current = true
			AND e.entity_type = 'track'
			AND t.access_authorities IS NULL
		WHERE ` + strings.Join(filters, " AND ") + `
		ORDER BY
			CASE WHEN e.end_date IS NULL OR e.end_date > NOW() THEN 0 ELSE 1 END ASC,
			CASE WHEN e.end_date IS NULL OR e.end_date > NOW() THEN e.end_date END ASC NULLS LAST,
			CASE WHEN e.end_date IS NOT NULL AND e.end_date <= NOW() THEN e.end_date END DESC,
			e.event_id ASC
		LIMIT @limit OFFSET @offset;
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"limit":  params.Limit,
		"offset": params.Offset,
	})
	if err != nil {
		return err
	}
	defer rows.Close()

	var items []dbv1.GetEventsRow
	for rows.Next() {
		var row dbv1.GetEventsRow
		if err := rows.Scan(
			&row.EventID,
			&row.EntityType,
			&row.UserID,
			&row.EntityID,
			&row.EventType,
			&row.EndDate,
			&row.IsDeleted,
			&row.CreatedAt,
			&row.UpdatedAt,
			&row.EventData,
		); err != nil {
			return err
		}
		items = append(items, row)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	data := make([]dbv1.FullEvent, 0, len(items))
	for _, event := range items {
		data = append(data, app.queries.ToFullEvent(event))
	}

	return c.JSON(fiber.Map{
		"data": data,
	})
}
