package api

import (
	"strings"

	"api.audius.co/api/dbv1"
	"api.audius.co/config"
	"api.audius.co/trashid"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetRemixContestsParams struct {
	Limit  int    `query:"limit" default:"25" validate:"min=1,max=100"`
	Offset int    `query:"offset" default:"0" validate:"min=0"`
	Status string `query:"status" default:"all" validate:"oneof=active ended all"`
}

// v1EventsRemixContests returns remix-contest events from the events table.
// Sort priority:
//  1. Open contests whose host is followed by the Audius account.
//  2. Open contests whose host is not followed by the Audius account.
//  3. Ended contests whose host is followed by the Audius account.
//  4. Ended contests whose host is not followed by the Audius account.
//
// The Audius account is config.Cfg.FeaturedAudienceUserID; 0 disables the
// follow-based tiebreak (groups collapse to open-vs-ended only).
// Within each group, results are sorted by entry count descending, then by
// the active-first / soonest-ending sort, then event_id ASC as a stable tiebreak.
// Supports pagination and an optional `status` filter (active | ended | all).
func (app *ApiServer) v1EventsRemixContests(c *fiber.Ctx) error {
	params := GetRemixContestsParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	filters := []string{
		"e.event_type = 'remix_contest'",
		"e.is_deleted = false",
		"u.is_deactivated = false",
		"u.is_available = true",
		"(e.entity_type != 'track' OR (t.track_id IS NOT NULL AND t.is_delete = false AND t.is_unlisted = false))",
		// Shadow-ban filters — two parallel community signals lifted from
		// v1_event_comments / v1_track_comments. A host disappears from the
		// discovery list if either:
		//   1. They authored a comment that crossed the high-karma-reporter
		//      threshold (sum of reporters' follower_count exceeds
		//      karmaCommentCountThreshold) — same threshold that hides the
		//      comment itself on comment endpoints, just lifted from
		//      comment_id to author user_id.
		//   2. They are in the karma-muted set (sum of muters'
		//      follower_count crosses karmaCommentCountThreshold).
		"e.user_id NOT IN (SELECT user_id FROM karma_reported_authors)",
		"e.user_id NOT IN (SELECT muted_user_id FROM muted_by_karma)",
	}

	switch params.Status {
	case "active":
		filters = append(filters, "(e.end_date IS NULL OR e.end_date > NOW())")
	case "ended":
		filters = append(filters, "(e.end_date IS NOT NULL AND e.end_date <= NOW())")
	}

	// LATERAL subquery mirrors the entry-count filter used in v1TrackRemixes
	// (only_contest_entries=true): a child track is an entry iff it was created
	// after the contest started, before its end_date, and is currently listed.
	sql := `
		WITH
		muted_by_karma AS (
			SELECT muted_user_id
			FROM muted_users
			JOIN aggregate_user ON muted_users.user_id = aggregate_user.user_id
			WHERE muted_users.is_delete = false
			GROUP BY muted_user_id
			HAVING SUM(aggregate_user.follower_count) >= @karmaCommentCountThreshold
		),
		-- Comments that crossed the high-karma reporter threshold — identical
		-- shape to high_karma_reporters in v1_track_comments / v1_event_comments.
		high_karma_reporters AS (
			SELECT comment_reports.comment_id
			FROM comment_reports
			JOIN aggregate_user ON comment_reports.user_id = aggregate_user.user_id
			WHERE comment_reports.is_delete = false
			GROUP BY comment_reports.comment_id
			HAVING SUM(aggregate_user.follower_count) >= @karmaCommentCountThreshold
		),
		-- Authors of any comment in high_karma_reporters. Lifts the per-comment
		-- shadow-ban signal up to the user level so the contest list hides
		-- hosts whose comments are already being hidden by the same threshold.
		karma_reported_authors AS (
			SELECT DISTINCT comments.user_id
			FROM comments
			JOIN high_karma_reporters ON high_karma_reporters.comment_id = comments.comment_id
		)
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
			e.event_data,
			CASE
				WHEN er.slug IS NOT NULL AND u.handle_lc IS NOT NULL
				THEN '/' || u.handle_lc || '/contest/' || er.slug
				ELSE NULL
			END AS permalink,
			COALESCE(ec.entry_count, 0) AS entry_count
		FROM events e
		JOIN users u ON u.user_id = e.user_id
			AND u.is_current = true
		LEFT JOIN event_routes er ON er.event_id = e.event_id AND er.is_current = true
		LEFT JOIN tracks t ON t.track_id = e.entity_id
			AND t.is_current = true
			AND e.entity_type = 'track'
			AND t.access_authorities IS NULL
		LEFT JOIN LATERAL (
			SELECT COUNT(DISTINCT ct.track_id) AS entry_count
			FROM remixes rm
			JOIN tracks ct ON ct.track_id = rm.child_track_id
			WHERE rm.parent_track_id = e.entity_id
				AND ct.is_current = true
				AND ct.is_delete = false
				AND ct.is_unlisted = false
				AND ct.created_at > e.created_at
				AND (e.end_date IS NULL OR ct.created_at < e.end_date)
		) ec ON true
		WHERE ` + strings.Join(filters, " AND ") + `
		ORDER BY
			-- Primary: open (0) before ended (1).
			CASE WHEN e.end_date IS NULL OR e.end_date > NOW() THEN 0 ELSE 1 END ASC,
			-- Secondary: Audius-followed hosts (0) before unfollowed (1).
			-- When @audius_user_id is 0 the tiebreak collapses (all rows score 1).
			CASE
				WHEN @audius_user_id::int4 != 0
					AND EXISTS (
						SELECT 1 FROM follows f
						WHERE f.follower_user_id = @audius_user_id::int4
							AND f.followee_user_id = e.user_id
							AND f.is_current = true
							AND f.is_delete = false
					) THEN 0
				ELSE 1
			END ASC,
			-- Within each group, more entries first.
			COALESCE(ec.entry_count, 0) DESC,
			CASE WHEN e.end_date IS NULL OR e.end_date > NOW() THEN e.end_date END ASC NULLS LAST,
			CASE WHEN e.end_date IS NOT NULL AND e.end_date <= NOW() THEN e.end_date END DESC,
			e.event_id ASC
		LIMIT @limit OFFSET @offset;
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"limit":                      params.Limit,
		"offset":                     params.Offset,
		"audius_user_id":             config.Cfg.FeaturedAudienceUserID,
		"karmaCommentCountThreshold": karmaCommentCountThreshold,
	})
	if err != nil {
		return err
	}
	defer rows.Close()

	var items []dbv1.GetEventsRow
	entryCounts := map[string]int64{}
	for rows.Next() {
		var row dbv1.GetEventsRow
		var entryCount int64
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
			&row.Permalink,
			&entryCount,
		); err != nil {
			return err
		}
		items = append(items, row)
		if row.EntityType == dbv1.EventEntityTypeTrack && row.EntityID.Valid {
			entryCounts[trashid.MustEncodeHashID(int(row.EntityID.Int32))] = entryCount
		}
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

	// Load tracks first so we can also resolve each track's owner id into the
	// related users list (contest host ≠ track owner isn't common in practice,
	// but we don't want the UI to make a second round-trip when it happens).
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

	// `entry_counts` was populated alongside the main query above (keyed by the
	// contest's parent track hashid) so the UI can prime the
	// `useRemixes({ trackId, isContestEntry: true })` cache directly.
	return c.JSON(fiber.Map{
		"data": data,
		"related": fiber.Map{
			"users":        users,
			"tracks":       tracks,
			"entry_counts": entryCounts,
		},
	})
}
