-- name: GetUserByHandle :one
SELECT * FROM users
WHERE handle_lc = $1 LIMIT 1;
