package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type GetUsersListenCountsMonthlyQueryParams struct {
	StartTime string `query:"start_time" validate:"datetime=2006-01-02"`
	EndTime   string `query:"end_time" validate:"datetime=2006-01-02"`
}

func (app *ApiServer) v1UsersListenCountsMonthly(c *fiber.Ctx) error {
	var params GetUsersListenCountsMonthlyQueryParams
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	sql := `
    SELECT
        play_item_id,
        timestamp,
        SUM(count) AS count
    FROM aggregate_monthly_plays
    WHERE play_item_id IN (
		SELECT track_id from tracks where owner_id = @userId
	)
    AND timestamp >= @startTime
    AND timestamp < @endTime
    GROUP BY play_item_id, timestamp
	;
	`

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"userId":    app.getUserId(c),
		"startTime": params.StartTime,
		"endTime":   params.EndTime,
	})
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
