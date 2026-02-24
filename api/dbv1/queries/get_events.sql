-- name: GetEvents :many
SELECT
  e.event_id AS event_id,
  e.entity_type::event_entity_type AS entity_type,
  e.user_id AS user_id,
  e.entity_id AS entity_id,
  e.event_type::event_type AS event_type,
  e.end_date AS end_date,
  e.is_deleted AS is_deleted,
  e.created_at AS created_at,
  e.updated_at AS updated_at,
  e.event_data AS event_data
FROM events e
LEFT JOIN tracks t ON t.track_id = e.entity_id AND t.is_current = true AND e.entity_type = 'track'
WHERE
  (@entity_ids::int[] = '{}' OR e.entity_id = ANY(@entity_ids::int[]))
  AND (@event_ids::int[] = '{}' OR e.event_id = ANY(@event_ids::int[]))
  AND (@entity_type::text = '' OR e.entity_type = @entity_type::event_entity_type)
  AND (@event_type::text = '' OR e.event_type = @event_type::event_type)
  AND (@filter_deleted::boolean IS NULL OR e.is_deleted = @filter_deleted)
  AND (e.entity_type != 'track' OR (t.track_id IS NOT NULL AND t.is_delete = false))
ORDER BY e.created_at DESC, e.event_id ASC
LIMIT @limit_val
OFFSET @offset_val;
