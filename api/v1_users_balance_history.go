package api

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5"
)

type Granularity string

const (
	GranularityHourly Granularity = "hourly"
	GranularityDaily  Granularity = "daily"
)

type BalanceHistoryQueryParams struct {
	StartTime   *time.Time  `query:"start_time"`
	EndTime     *time.Time  `query:"end_time"`
	Granularity Granularity `query:"granularity" default:"hourly"`
}

// SetDefaults sets default time range if not provided (last 7 days from now)
// and default granularity if not provided
func (p *BalanceHistoryQueryParams) SetDefaults() {
	now := time.Now()
	if p.StartTime == nil {
		startTime := now.Add(-7 * 24 * time.Hour)
		p.StartTime = &startTime
	}
	if p.EndTime == nil {
		p.EndTime = &now
	}
	if p.Granularity == "" {
		p.Granularity = GranularityHourly
	}
}

// Validate validates the query parameters
func (p *BalanceHistoryQueryParams) Validate() error {
	if p.Granularity != GranularityHourly && p.Granularity != GranularityDaily {
		return fiber.NewError(fiber.StatusBadRequest, "granularity must be 'hourly' or 'daily'")
	}
	return nil
}

// GetTimeRange returns the resolved time range and validates it
func (p *BalanceHistoryQueryParams) GetTimeRange() (startTime, endTime time.Time, err error) {
	startTime = *p.StartTime
	endTime = *p.EndTime

	if endTime.Before(startTime) {
		return time.Time{}, time.Time{}, fiber.NewError(fiber.StatusBadRequest, "end_time must be after start_time")
	}

	return startTime, endTime, nil
}

type BalanceHistoryDataPoint struct {
	Timestamp  int64   `json:"timestamp"`
	BalanceUsd float64 `json:"balance_usd"`
}

type balanceHistoryRow struct {
	Timestamp  time.Time `json:"timestamp"`
	BalanceUsd float64   `json:"balance_usd"`
}

type BalanceHistoryResponse struct {
	Data []BalanceHistoryDataPoint `json:"data"`
}

// v1UsersBalanceHistory returns the historical balance data for a user
// Granularity can be 'hourly' or 'daily' (defaults to 'hourly')
func (app *ApiServer) v1UsersBalanceHistory(c *fiber.Ctx) error {
	// Get user ID from middleware (set by requireUserIdMiddleware)
	userId := app.getUserId(c)

	// Parse and validate query params
	params := BalanceHistoryQueryParams{}
	if err := app.ParseAndValidateQueryParams(c, &params); err != nil {
		return err
	}

	// Set dynamic defaults (cannot use struct tags for time.Now()-based defaults)
	params.SetDefaults()

	// Validate granularity
	if err := params.Validate(); err != nil {
		return err
	}

	// Get validated time range (validates that end_time is after start_time)
	startTime, endTime, err := params.GetTimeRange()
	if err != nil {
		return err
	}

	// First sum balance_usd across mints at each hourly snapshot to get the
	// total portfolio value per hour.
	sql := `
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

	if params.Granularity == GranularityDaily {
		// For daily, pick the latest hour within each day (end-of-day balance).
		// Do not sum balances across hours, since each hour is a point-in-time
		// snapshot, not an additive quantity.
		sql = `
			WITH hourly_totals AS (
				SELECT
					timestamp,
					SUM(balance_usd) AS balance_usd
				FROM user_balance_history
				WHERE user_id = @user_id
					AND timestamp >= @start_time
					AND timestamp <= @end_time
				GROUP BY timestamp
			)
			SELECT DISTINCT ON (date_trunc('day', timestamp))
				date_trunc('day', timestamp) AS timestamp,
				balance_usd
			FROM hourly_totals
			ORDER BY date_trunc('day', hourly_totals.timestamp) ASC, hourly_totals.timestamp DESC
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

	// Collect rows using pgx.CollectRows
	historyRows, err := pgx.CollectRows(rows, pgx.RowToStructByName[balanceHistoryRow])
	if err != nil {
		return err
	}

	// Transform to response format (convert timestamp to Unix)
	data := make([]BalanceHistoryDataPoint, len(historyRows))
	for i, row := range historyRows {
		data[i] = BalanceHistoryDataPoint{
			Timestamp:  row.Timestamp.Unix(),
			BalanceUsd: row.BalanceUsd,
		}
	}

	return c.JSON(BalanceHistoryResponse{
		Data: data,
	})
}
