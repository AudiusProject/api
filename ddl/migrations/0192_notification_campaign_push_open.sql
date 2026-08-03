-- Run against Discovery Postgres (same DB as API read/write pools).
-- Idempotent: safe to run once in environments that do not use full 01_schema dumps.

CREATE TABLE IF NOT EXISTS public.notification_campaign_push_open (
    campaign_id uuid NOT NULL,
    user_id integer NOT NULL,
    opened_at timestamp with time zone DEFAULT now() NOT NULL
);

DO $$
BEGIN
  IF NOT EXISTS (
    SELECT 1 FROM pg_constraint
    WHERE conname = 'notification_campaign_push_open_pkey'
  ) THEN
    ALTER TABLE ONLY public.notification_campaign_push_open
      ADD CONSTRAINT notification_campaign_push_open_pkey
      PRIMARY KEY (campaign_id, user_id);
  END IF;
END $$;
