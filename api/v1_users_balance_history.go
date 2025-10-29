package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type BalanceHistoryQueryParams struct {
	StartTime *time.Time `query:"start_time"`
	EndTime   *time.Time `query:"end_time"`
}

type BalanceHistoryDataPoint struct {
	Timestamp  int64   `json:"timestamp"`
	BalanceUsd float64 `json:"balance_usd"`
}

type BalanceHistoryResponse struct {
	Data []BalanceHistoryDataPoint `json:"data"`
}

// v1UsersBalanceHistory returns the historical balance data for a user
// Returns hourly data for ≤7 days, daily data for >7 days
func (app *ApiServer) v1UsersBalanceHistory(c *fiber.Ctx) error {
	// Get user ID from middleware (set by requireUserIdMiddleware)
	userId := app.getUserId(c)

	// Parse query params
	params := BalanceHistoryQueryParams{}
	if err := c.QueryParser(&params); err != nil {
		return fiber.NewError(fiber.StatusBadRequest, "invalid query parameters")
	}

	// Set default time range if not provided (last 7 days)
	now := time.Now()
	endTime := now
	startTime := now.Add(-7 * 24 * time.Hour)

	if params.StartTime != nil {
		startTime = *params.StartTime
	}
	if params.EndTime != nil {
		endTime = *params.EndTime
	}

	// Validate time range
	if endTime.Before(startTime) {
		return fiber.NewError(fiber.StatusBadRequest, "end_time must be after start_time")
	}

	// Determine granularity based on time range
	duration := endTime.Sub(startTime)
	isWeekOrLess := duration <= 7*24*time.Hour

	var sql string
	if isWeekOrLess {
		// Hourly granularity for week or less
		sql = `
			SELECT
				timestamp,
				SUM(balance_usd) AS balance_usd
			FROM user_balance_history
			WHERE user_id = @user_id
				AND timestamp >= @start_time
				AND timestamp <= @end_time
			GROUP BY timestamp
			ORDER BY timestamp ASC
		`
	} else {
		// Daily granularity for more than a week
		sql = `
			SELECT
				date_trunc('day', timestamp) AS timestamp,
				SUM(balance_usd) AS balance_usd
			FROM user_balance_history
			WHERE user_id = @user_id
				AND timestamp >= @start_time
				AND timestamp <= @end_time
			GROUP BY date_trunc('day', timestamp)
			ORDER BY date_trunc('day', timestamp) ASC
		`
	}

	rows, err := app.pool.Query(c.Context(), sql, pgx.NamedArgs{
		"user_id":    userId,
		"start_time": startTime,
		"end_time":   endTime,
	})
	if err != nil {
		return err
	}
	defer rows.Close()

	// Collect results
	var data []BalanceHistoryDataPoint
	for rows.Next() {
		var timestamp time.Time
		var balanceUsd float64

		if err := rows.Scan(&timestamp, &balanceUsd); err != nil {
			return err
		}

		data = append(data, BalanceHistoryDataPoint{
			Timestamp:  timestamp.Unix(),
			BalanceUsd: balanceUsd,
		})
	}

	if err := rows.Err(); err != nil {
		return err
	}

	return c.JSON(BalanceHistoryResponse{
		Data: data,
	})
}
