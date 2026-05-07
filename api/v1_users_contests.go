package api

import (
	"strings"

	"api.audius.co/api/dbv1"
	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUserContestsParams struct {
	Limit  int    `query:"limit" default:"25" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0"`
	Status string `query:"status" default:"all" validate:"oneof=active ended all"`
}

// v1UserContests returns remix-contest events hosted by a specific user,
// ordered with currently-active contests first (by soonest-ending end_date),
// followed by ended contests (most-recently-ended first). Mirrors
// v1EventsRemixContests but scoped to events where event.user_id matches the
// requested profile, so the discovery client doesn't have to fetch the global
// list and filter client-side.
func (app *ApiServer) v1UserContests(c *fiber.Ctx) error {
	params := GetUserContestsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	userId := app.getUserId(c)

	filters := []string{
		"e.event_type = 'remix_contest'",
		"e.is_deleted = false",
		"e.user_id = @user_id",
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
		"user_id": userId,
		"limit":   params.Limit,
		"offset":  params.Offset,
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
	trackIDs := make([]int32, 0, len(items))
	userIDSet := map[int32]struct{}{}
	for _, event := range items {
		data = append(data, app.queries.ToFullEvent(event))
		if event.EntityType == dbv1.EventEntityTypeTrack && event.EntityID.Valid {
			trackIDs = append(trackIDs, event.EntityID.Int32)
		}
		userIDSet[event.UserID] = struct{}{}
	}

	myID := app.getMyId(c)
	authedWallet := app.tryGetAuthedWallet(c)

	var trackMap map[int32]dbv1.Track
	if len(trackIDs) > 0 {
		trackMap, err = app.queries.TracksKeyed(c.Context(), dbv1.TracksParams{
			GetTracksParams: dbv1.GetTracksParams{
				Ids:          trackIDs,
				MyID:         myID,
				AuthedWallet: authedWallet,
			},
		})
		if err != nil {
			return err
		}
	}
	for _, t := range trackMap {
		userIDSet[t.GetTracksRow.UserID] = struct{}{}
	}

	userIDs := make([]int32, 0, len(userIDSet))
	for id := range userIDSet {
		userIDs = append(userIDs, id)
	}
	var userMap map[int32]dbv1.User
	if len(userIDs) > 0 {
		userMap, err = app.queries.UsersKeyed(c.Context(), dbv1.GetUsersParams{
			Ids:  userIDs,
			MyID: myID,
		})
		if err != nil {
			return err
		}
	}

	users := make([]dbv1.User, 0, len(userMap))
	for _, u := range userMap {
		users = append(users, u)
	}
	tracks := make([]dbv1.Track, 0, len(trackMap))
	for _, t := range trackMap {
		tracks = append(tracks, t)
	}

	// Per-contest entry counts. Mirrors v1EventsRemixContests so the
	// discovery client can prime useRemixes({ trackId, isContestEntry: true })
	// from the same payload.
	entryCounts := map[string]int64{}
	if len(trackIDs) > 0 {
		countRows, err := app.pool.Query(c.Context(), `
			SELECT
				e.entity_id,
				COUNT(DISTINCT t.track_id) FILTER (
					WHERE t.is_current = true
					AND t.is_delete = false
					AND t.is_unlisted = false
					AND t.created_at > e.created_at
					AND (e.end_date IS NULL OR t.created_at < e.end_date)
				) AS entry_count
			FROM events e
			LEFT JOIN remixes rm ON rm.parent_track_id = e.entity_id
			LEFT JOIN tracks t ON t.track_id = rm.child_track_id
			WHERE e.event_type = 'remix_contest'
				AND e.is_deleted = false
				AND e.entity_type = 'track'
				AND e.entity_id = ANY(@track_ids)
			GROUP BY e.entity_id
		`, pgx.NamedArgs{"track_ids": trackIDs})
		if err != nil {
			return err
		}
		defer countRows.Close()
		for countRows.Next() {
			var parentTrackID int32
			var count int64
			if err := countRows.Scan(&parentTrackID, &count); err != nil {
				return err
			}
			entryCounts[trashid.MustEncodeHashID(int(parentTrackID))] = count
		}
		if err := countRows.Err(); err != nil {
			return err
		}
	}

	return c.JSON(fiber.Map{
		"data": data,
		"related": fiber.Map{
			"users":        users,
			"tracks":       tracks,
			"entry_counts": entryCounts,
		},
	})
}
