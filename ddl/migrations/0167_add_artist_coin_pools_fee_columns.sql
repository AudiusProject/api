DO $$
BEGIN
    ALTER TABLE artist_coin_pools ADD COLUMN IF NOT EXISTS total_trading_quote_fee NUMERIC;
    ALTER TABLE artist_coin_pools ADD COLUMN IF NOT EXISTS total_protocol_quote_fee NUMERIC;
END
$$;
