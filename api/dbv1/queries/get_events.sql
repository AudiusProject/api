-- name: GetEvents :many
SELECT
  event_id,
  entity_id,
  entity_type,
  event_type,
  blocknumber,
  created_at,
  updated_at,
  is_deleted,
  metadata
FROM events
WHERE 
  -- Filter by entity_id if provided
  (@entity_id::int IS NULL OR entity_id = @entity_id)
  -- Filter by entity_type if provided
  AND (@entity_type::text IS NULL OR entity_type = @entity_type)
  -- Filter by event_type if provided
  AND (@event_type::text IS NULL OR event_type = @event_type)
  -- Filter deleted events by default
  AND (@filter_deleted::boolean IS NULL OR is_deleted = @filter_deleted)
ORDER BY created_at DESC
LIMIT @limit
OFFSET @offset;
