-- Accepted collaborators for a set of tracks, used to embed a `collaborators`
-- array on track responses. Returns one row per (track, collaborator); the Go
-- layer bulk-resolves the user objects. Backed by the track_collaborators
-- primary key (track_id leads), so the ANY(...) lookup is index-served.
-- name: GetTrackCollaborators :many
SELECT track_id, collaborator_user_id
FROM track_collaborators
WHERE track_id = ANY(@track_ids::int[])
  AND status = 'accepted'
ORDER BY track_id, created_at;

-- A user's collaborator invites/credits, optionally filtered by status
-- (pending/accepted/rejected). Backed by the covering
-- (collaborator_user_id, status, track_id) index.
-- name: GetTrackCollaboratorInvitesForUser :many
SELECT track_id, collaborator_user_id, invited_by, status, created_at, updated_at
FROM track_collaborators
WHERE collaborator_user_id = @user_id::int
  AND (sqlc.narg('status')::text IS NULL OR status = sqlc.narg('status'))
ORDER BY created_at DESC;
