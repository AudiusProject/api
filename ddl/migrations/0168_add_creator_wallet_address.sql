DO $$
BEGIN
    ALTER TABLE artist_coin_pools ADD COLUMN IF NOT EXISTS creator_wallet_address TEXT;
END
$$;

