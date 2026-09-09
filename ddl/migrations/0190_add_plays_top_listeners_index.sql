BEGIN;

-- Add index optimized for top_listeners query
-- This index allows efficient filtering by play_item_id and supports
-- the count(distinct date_trunc('hour', created_at)) operation
CREATE INDEX IF NOT EXISTS ix_plays_play_item_user_date 
ON plays(play_item_id, user_id, created_at) 
WHERE user_id IS NOT NULL;

COMMIT;
