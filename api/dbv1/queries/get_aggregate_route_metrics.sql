-- name: GetAggregateRouteMetrics :many
WITH week_unique_daily AS (
    SELECT 
        timestamp,
        count AS unique_count,
        summed_count AS summed_unique_count
    FROM 
        aggregate_daily_unique_users_metrics
    WHERE 
        timestamp >= CURRENT_DATE - INTERVAL '7 days'
        AND timestamp < CURRENT_DATE
),
week_total_daily AS (
    SELECT 
        timestamp,
        count AS total_count
    FROM 
        aggregate_daily_total_users_metrics
    WHERE 
        timestamp >= CURRENT_DATE - INTERVAL '7 days'
        AND timestamp < CURRENT_DATE
),
month_unique_daily AS (
    SELECT 
        timestamp,
        count AS unique_count,
        summed_count AS summed_unique_count
    FROM 
        aggregate_daily_unique_users_metrics
    WHERE 
        timestamp >= CURRENT_DATE - INTERVAL '30 days'
        AND timestamp < CURRENT_DATE
),
month_total_daily AS (
    SELECT 
        timestamp,
        count AS total_count
    FROM 
        aggregate_daily_total_users_metrics
    WHERE 
        timestamp >= CURRENT_DATE - INTERVAL '30 days'
        AND timestamp < CURRENT_DATE
),
month_unique_weekly AS (
    SELECT 
        date_trunc('week', timestamp)::timestamp as timestamp,
        SUM(count) AS unique_count,
        SUM(summed_count) AS summed_unique_count
    FROM 
        aggregate_daily_unique_users_metrics
    WHERE 
        timestamp >= CURRENT_DATE - INTERVAL '30 days'
        AND timestamp < CURRENT_DATE
    GROUP BY 
        date_trunc('week', timestamp)
),
month_total_weekly AS (
    SELECT 
        date_trunc('week', timestamp)::timestamp as timestamp,
        SUM(count) AS total_count
    FROM 
        aggregate_daily_total_users_metrics
    WHERE 
        timestamp >= CURRENT_DATE - INTERVAL '30 days'
        AND timestamp < CURRENT_DATE
    GROUP BY 
        date_trunc('week', timestamp)
),
year_unique_monthly AS (
    SELECT 
        timestamp,
        count AS unique_count,
        summed_count AS summed_unique_count
    FROM 
        aggregate_monthly_unique_users_metrics
    WHERE 
        timestamp >= CURRENT_DATE - INTERVAL '365 days'
        AND timestamp < date_trunc('month', CURRENT_DATE)
),
year_total_monthly AS (
    SELECT 
        timestamp,
        count AS total_count
    FROM 
        aggregate_monthly_total_users_metrics
    WHERE 
        timestamp >= CURRENT_DATE - INTERVAL '365 days'
        AND timestamp < date_trunc('month', CURRENT_DATE)
),
alltime_unique_monthly AS (
    SELECT 
        timestamp,
        count AS unique_count,
        summed_count AS summed_unique_count
    FROM 
        aggregate_monthly_unique_users_metrics
    WHERE 
        timestamp < date_trunc('month', CURRENT_DATE)
),
alltime_total_monthly AS (
    SELECT 
        timestamp,
        count AS total_count
    FROM 
        aggregate_monthly_total_users_metrics
    WHERE 
        timestamp < date_trunc('month', CURRENT_DATE)
),
alltime_unique_weekly AS (
    SELECT 
        date_trunc('week', timestamp)::timestamp as timestamp,
        SUM(count) AS unique_count,
        SUM(summed_count) AS summed_unique_count
    FROM 
        aggregate_daily_unique_users_metrics
    WHERE 
        timestamp < CURRENT_DATE
    GROUP BY 
        date_trunc('week', timestamp)
),
alltime_total_weekly AS (
    SELECT 
        date_trunc('week', timestamp)::timestamp as timestamp,
        SUM(count) AS total_count
    FROM 
        aggregate_daily_total_users_metrics
    WHERE 
        timestamp < CURRENT_DATE
    GROUP BY 
        date_trunc('week', timestamp)
),
result_data AS (
    -- Week with day bucket size
    SELECT 
        w_unique.timestamp,
        w_unique.unique_count,
        w_unique.summed_unique_count,
        w_total.total_count
    FROM 
        week_unique_daily w_unique
    JOIN 
        week_total_daily w_total 
    ON 
        w_unique.timestamp = w_total.timestamp
    WHERE 
        @time_range = 'week' AND @bucket_size = 'day'
    
    UNION ALL
    
    -- Month with day bucket size
    SELECT 
        m_unique.timestamp,
        m_unique.unique_count,
        m_unique.summed_unique_count,
        m_total.total_count
    FROM 
        month_unique_daily m_unique
    JOIN 
        month_total_daily m_total 
    ON 
        m_unique.timestamp = m_total.timestamp
    WHERE 
        @time_range = 'month' AND @bucket_size = 'day'
    
    UNION ALL
    
    -- Month with week bucket size
    SELECT 
        m_unique.timestamp,
        m_unique.unique_count,
        m_unique.summed_unique_count,
        m_total.total_count
    FROM 
        month_unique_weekly m_unique
    JOIN 
        month_total_weekly m_total 
    ON 
        m_unique.timestamp = m_total.timestamp
    WHERE 
        @time_range = 'month' AND @bucket_size = 'week'
    
    UNION ALL
    
    -- Year with month bucket size
    SELECT 
        y_unique.timestamp,
        y_unique.unique_count,
        y_unique.summed_unique_count,
        y_total.total_count
    FROM 
        year_unique_monthly y_unique
    JOIN 
        year_total_monthly y_total 
    ON 
        y_unique.timestamp = y_total.timestamp
    WHERE 
        @time_range = 'year' AND @bucket_size = 'month'
    
    UNION ALL
    
    -- All time with month bucket size
    SELECT 
        a_unique.timestamp,
        a_unique.unique_count,
        a_unique.summed_unique_count,
        a_total.total_count
    FROM 
        alltime_unique_monthly a_unique
    JOIN 
        alltime_total_monthly a_total 
    ON 
        a_unique.timestamp = a_total.timestamp
    WHERE 
        @time_range = 'all_time' AND @bucket_size = 'month'
    
    UNION ALL
    
    -- All time with week bucket size
    SELECT 
        a_unique.timestamp,
        a_unique.unique_count,
        a_unique.summed_unique_count,
        a_total.total_count
    FROM 
        alltime_unique_weekly a_unique
    JOIN 
        alltime_total_weekly a_total 
    ON 
        a_unique.timestamp = a_total.timestamp
    WHERE 
        @time_range = 'all_time' AND @bucket_size = 'week'
)
SELECT 
    -- Return the date as a formatted string in YYYY-MM-DD format
    to_char(date_trunc('month', timestamp)::date, 'YYYY-MM-DD') AS timestamp,
    unique_count,
    summed_unique_count,
    total_count
FROM 
    result_data
ORDER BY 
    timestamp ASC; 