begin;

-- Create index for plays by user ID and hourly timestamp
CREATE INDEX IF NOT EXISTS ix_plays_user_hour ON plays (user_id, date_trunc('hour', created_at)) WHERE user_id IS NOT NULL;

COMMIT;