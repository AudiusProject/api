package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersListenCountsMonthlyQueryParams struct {
	StartTime string `query:"start_time"`
	EndTime   string `query:"end_time"`
}

func (app *ApiServer) v1UsersListenCountsMonthly(c *fiber.Ctx) error {
	var params GetUsersListenCountsMonthlyQueryParams
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	userId := app.getUserId(c)

	// See v1_users_tracks.go: fold the (usually empty) collaborator set in via an
	// explicit id array only when present, so the common case keeps its owner-only
	// plan rather than seq-scanning tracks.
	collabRows, err := app.pool.Query(c.Context(),
		`SELECT track_id FROM track_collaborators WHERE collaborator_user_id = $1 AND status = 'accepted'`,
		userId)
	if err != nil {
		return err
	}
	collabTrackIds, err := pgx.CollectRows(collabRows, pgx.RowTo[int32])
	if err != nil {
		return err
	}

	ownerFilter := "owner_id = @userId"
	if len(collabTrackIds) > 0 {
		ownerFilter = "(owner_id = @userId OR track_id = ANY(@collab_track_ids))"
	}

	sql := `
    SELECT
        play_item_id,
        timestamp,
        SUM(count) AS count
    FROM aggregate_monthly_plays
    WHERE play_item_id IN (
		SELECT track_id FROM tracks WHERE stem_of IS NULL
			AND ` + ownerFilter + `
			AND (access_authorities IS NULL
			  OR (COALESCE(@authed_wallet, '') <> ''
			      AND EXISTS (SELECT 1 FROM unnest(access_authorities) aa WHERE lower(aa) = lower(@authed_wallet))))
	)
    AND timestamp >= @startTime
    AND timestamp < @endTime
    GROUP BY play_item_id, timestamp
	;
	`

	args := pgx.NamedArgs{
		"userId":        userId,
		"startTime":     params.StartTime,
		"endTime":       params.EndTime,
		"authed_wallet": app.tryGetAuthedWallet(c),
	}
	if len(collabTrackIds) > 0 {
		args["collab_track_ids"] = collabTrackIds
	}

	rows, err := app.pool.Query(c.Context(), sql, args)
	if err != nil {
		return err
	}

	type ListenCount struct {
		PlayItemId int       `db:"play_item_id" json:"trackId"`
		Timestamp  time.Time `db:"timestamp" json:"date"`
		Count      int64     `db:"count" json:"listens"`
	}
	counts, err := pgx.CollectRows(rows, pgx.RowToStructByName[ListenCount])
	if err != nil {
		return err
	}

	type MonthlyListenCount struct {
		TotalListens int64         `json:"totalListens"`
		TrackIDs     []int         `json:"trackIds"`
		ListenCounts []ListenCount `json:"listenCounts"`
	}

	byMonth := make(map[string]MonthlyListenCount)
	for _, count := range counts {
		month := count.Timestamp.Format("2006-01") + "-01T00:00:00Z"
		if _, exists := byMonth[month]; !exists {
			byMonth[month] = MonthlyListenCount{
				TotalListens: 0,
				TrackIDs:     []int{},
				ListenCounts: []ListenCount{},
			}
		}
		mlc := byMonth[month]
		mlc.TotalListens += count.Count
		mlc.TrackIDs = append(mlc.TrackIDs, int(count.PlayItemId))
		mlc.ListenCounts = append(mlc.ListenCounts, count)
		byMonth[month] = mlc
	}

	return c.JSON(fiber.Map{
		"data": byMonth,
	})
}
