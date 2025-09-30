DO $$
BEGIN
    ALTER TABLE artist_coins ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP DEFAULT NOW();
    ALTER TABLE artist_coins ADD COLUMN IF NOT EXISTS x_handle TEXT;
    ALTER TABLE artist_coins ADD COLUMN IF NOT EXISTS instagram_handle TEXT;
    ALTER TABLE artist_coins ADD COLUMN IF NOT EXISTS tiktok_handle TEXT;
    ALTER TABLE artist_coins ADD COLUMN IF NOT EXISTS website TEXT;
END
$$;
