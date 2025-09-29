DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'artist_coins'
            AND column_name = 'updated_at'
            AND table_schema = 'public'
    ) THEN
        ALTER TABLE artist_coins ADD COLUMN updated_at TIMESTAMP DEFAULT NOW();
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'artist_coins'
            AND column_name = 'twitter'
            AND table_schema = 'public'
    ) THEN
        ALTER TABLE artist_coins ADD COLUMN twitter TEXT;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'artist_coins'
            AND column_name = 'instagram'
            AND table_schema = 'public'
    ) THEN
        ALTER TABLE artist_coins ADD COLUMN instagram TEXT;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'artist_coins'
            AND column_name = 'tiktok'
            AND table_schema = 'public'
    ) THEN
        ALTER TABLE artist_coins ADD COLUMN tiktok TEXT;
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM information_schema.columns
        WHERE table_name = 'artist_coins'
            AND column_name = 'website'
            AND table_schema = 'public'
    ) THEN
        ALTER TABLE artist_coins ADD COLUMN website TEXT;
    END IF;
END
$$;
