-- track_collaborators is created natively by the OpenAudio ETL (go-openaudio
-- migration 0033). We mirror it here with IF NOT EXISTS so that:
--   1. fresh / test databases (which load sql/01_schema.sql, not the ETL
--      migrations) have the table, and
--   2. it exists before the functions/ pass runs, so the notification trigger
--      in handle_track_collaborator.sql always has its table.
-- The ETL migration and this one are byte-compatible CREATE TABLE IF NOT EXISTS
-- statements, so whichever runs first wins and the other is a no-op.
begin;

CREATE TABLE IF NOT EXISTS track_collaborators (
  track_id integer NOT NULL,
  collaborator_user_id integer NOT NULL,
  invited_by integer NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  created_at timestamp without time zone NOT NULL,
  updated_at timestamp without time zone NOT NULL,
  txhash character varying NOT NULL,
  blocknumber integer,
  CONSTRAINT track_collaborators_pkey PRIMARY KEY (track_id, collaborator_user_id),
  CONSTRAINT track_collaborators_status_check CHECK (status IN ('pending', 'accepted', 'rejected'))
);

CREATE INDEX IF NOT EXISTS idx_track_collaborators_collaborator
  ON track_collaborators (collaborator_user_id, status, track_id);

COMMENT ON TABLE track_collaborators IS 'Collaborator credits on a track. Owner invites via track metadata (status=pending); the collaborator accepts/declines on-chain (accepted/rejected). Indexed by ETL (go-openaudio).';

COMMIT;
