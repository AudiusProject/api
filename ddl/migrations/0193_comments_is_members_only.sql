-- Add is_members_only flag to fan club text posts.
-- Default false so track comments (the vast majority) are unaffected.
-- Fan club post creation explicitly sets this to true when the artist
-- has the "Members Only" toggle enabled.

ALTER TABLE public.comments
ADD COLUMN IF NOT EXISTS is_members_only boolean NOT NULL DEFAULT false;

-- Back-fill existing fan club text posts as members-only (preserving
-- current behaviour where all fan club posts were gated).
UPDATE public.comments
SET is_members_only = true
WHERE entity_type = 'FanClub';
