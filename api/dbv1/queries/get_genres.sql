-- name: GetGenres :many
SELECT
    genre,
    COUNT(track_id) AS count
FROM
    tracks
WHERE
    genre IS NOT NULL
    AND genre != ''
    AND is_current = TRUE
    AND created_at > @start_time
GROUP BY
    genre
ORDER BY
    count DESC
LIMIT @limit_val
OFFSET @offset_val;
