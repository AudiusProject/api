-- name: GetAggregateRouteMetricsTrailing :one
-- Returns aggregate metrics for trailing period (week, month, or year)
WITH date_filter AS (
    -- Define the date range based on time_range parameter
    SELECT 
        CASE
            WHEN @time_range = 'year' THEN CURRENT_DATE - INTERVAL '365 days'
            WHEN @time_range = 'week' THEN CURRENT_DATE - INTERVAL '7 days'
            ELSE CURRENT_DATE - INTERVAL '30 days' -- default to month
        END AS start_date,
        CURRENT_DATE AS end_date
),
unique_counts AS (
    -- Get unique and summed unique counts for the period
    SELECT
        SUM(count) AS unique_count,
        SUM(summed_count) AS summed_unique_count
    FROM
        aggregate_daily_unique_users_metrics, date_filter
    WHERE
        timestamp >= date_filter.start_date
        AND timestamp < date_filter.end_date
),
total_counts AS (
    -- Get total counts for the period
    SELECT
        SUM(count) AS total_count
    FROM
        aggregate_daily_total_users_metrics, date_filter
    WHERE
        timestamp >= date_filter.start_date
        AND timestamp < date_filter.end_date
)
-- Return a single row with all aggregated metrics
SELECT
    unique_counts.unique_count,
    unique_counts.summed_unique_count,
    total_counts.total_count
FROM
    unique_counts, total_counts; 