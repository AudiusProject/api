-- name: GetEvents :many
SELECT
  event_id,
  entity_type::event_entity_type,
  user_id,
  entity_id,
  event_type::event_type,
  end_date,
  is_deleted,
  created_at,
  updated_at,
  event_data
FROM events
WHERE
  (@entity_ids::int[] = '{}' OR entity_id = ANY(@entity_ids::int[]))
  AND (@event_ids::int[] = '{}' OR event_id = ANY(@event_ids::int[]))
  AND (@entity_type::text = '' OR entity_type = @entity_type::event_entity_type)
  AND (@event_type::text = '' OR event_type = @event_type::event_type)
  AND (@filter_deleted::boolean IS NULL OR is_deleted = @filter_deleted)
ORDER BY created_at DESC, event_id ASC
LIMIT @limit_val
OFFSET @offset_val;
