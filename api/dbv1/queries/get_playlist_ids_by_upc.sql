-- name: GetPlaylistIdsByUPC :many
SELECT playlist_id
FROM playlists
WHERE upc = ANY(@upcs::text[]);