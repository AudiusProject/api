-- name: GetTrackIdsByPermalink :many
SELECT track_id
FROM track_routes
JOIN users ON users.user_id = track_routes.owner_id
WHERE handle_lc = ANY(@handles::text[])
    AND slug = ANY(@slugs::text[])
	AND CONCAT(handle_lc, '/', slug) = ANY(@permalinks::text[]) -- in case of conflicts across users
;