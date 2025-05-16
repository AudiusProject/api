-- name: GetPlaysMetrics :many
SELECT
    date_trunc(@bucket_size, hourly_timestamp)::timestamp AS timestamp,
    SUM(play_count) AS count
FROM
    hourly_play_counts
WHERE
    hourly_timestamp > @start_time
GROUP BY
    date_trunc(@bucket_size, hourly_timestamp)
ORDER BY
    timestamp DESC
LIMIT @limit_val; 