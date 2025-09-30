DO $$
BEGIN
    ALTER TABLE artist_coins DROP COLUMN IF EXISTS x_handle;
    ALTER TABLE artist_coins DROP COLUMN IF EXISTS instagram_handle;
    ALTER TABLE artist_coins DROP COLUMN IF EXISTS tiktok_handle;
    ALTER TABLE artist_coins DROP COLUMN IF EXISTS website;
    ALTER TABLE artist_coins ADD COLUMN IF NOT EXISTS link_1 TEXT;
    ALTER TABLE artist_coins ADD COLUMN IF NOT EXISTS link_2 TEXT;
    ALTER TABLE artist_coins ADD COLUMN IF NOT EXISTS link_3 TEXT;
    ALTER TABLE artist_coins ADD COLUMN IF NOT EXISTS link_4 TEXT;
END
$$;
