-- Add event_routes table to support permalink generation for events.
--
-- Mirrors the shape of track_routes / playlist_routes: slug is set by the
-- indexer when an event is created, owner_id points to the event's host user,
-- and is_current flags the canonical row so a LEFT JOIN ON is_current = true
-- always lands on at most one row per event.
CREATE TABLE IF NOT EXISTS public.event_routes (
    slug         character varying NOT NULL,
    owner_id     integer           NOT NULL,
    event_id     integer           NOT NULL,
    is_current   boolean           NOT NULL,
    blockhash    character varying NOT NULL,
    blocknumber  integer           NOT NULL,
    txhash       character varying NOT NULL,
    CONSTRAINT event_routes_pkey PRIMARY KEY (owner_id, slug)
);
CREATE INDEX IF NOT EXISTS event_routes_event_id_idx
    ON public.event_routes USING btree (event_id);
