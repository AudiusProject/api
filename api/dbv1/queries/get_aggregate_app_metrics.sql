-- name: GetAggregateAppMetrics :many
WITH week_data AS (
    SELECT 
        application_name AS name,
        SUM(count) AS count
    FROM 
        aggregate_daily_app_name_metrics
    WHERE 
        timestamp >= CURRENT_DATE - INTERVAL '7 days'
        AND timestamp < CURRENT_DATE
    GROUP BY 
        application_name
),
month_data AS (
    SELECT 
        application_name AS name,
        SUM(count) AS count
    FROM 
        aggregate_daily_app_name_metrics
    WHERE 
        timestamp >= CURRENT_DATE - INTERVAL '30 days'
        AND timestamp < CURRENT_DATE
    GROUP BY 
        application_name
),
year_data AS (
    SELECT 
        application_name AS name,
        SUM(count) AS count
    FROM 
        aggregate_monthly_app_name_metrics
    WHERE 
        timestamp >= CURRENT_DATE - INTERVAL '365 days'
        AND timestamp < CURRENT_DATE
    GROUP BY 
        application_name
),
all_time_data AS (
    SELECT 
        application_name AS name,
        SUM(count) AS count
    FROM 
        aggregate_monthly_app_name_metrics
    WHERE 
        timestamp < CURRENT_DATE
    GROUP BY 
        application_name
),
time_range_data AS (
    SELECT name, count FROM week_data WHERE @time_range = 'week'
    UNION ALL
    SELECT name, count FROM month_data WHERE @time_range = 'month'
    UNION ALL
    SELECT name, count FROM year_data WHERE @time_range = 'year'
    UNION ALL
    SELECT name, count FROM all_time_data WHERE @time_range = 'all_time'
)
SELECT
    name,
    count
FROM
    time_range_data
ORDER BY
    count DESC,
    name ASC
LIMIT @limit_val; 