-- name: GetUserByHandle :one
SELECT * FROM users
WHERE handle_lc = lower($1) LIMIT 1;
