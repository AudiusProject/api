CREATE OR REPLACE FUNCTION handle_artist_coins_change()
RETURNS trigger AS $$
BEGIN
    IF (OLD.mint IS NULL AND NEW.mint IS NOT NULL) 
        OR (OLD.mint IS NOT NULL AND NEW.mint IS NULL)
        OR OLD.mint != NEW.mint
    THEN
        PERFORM pg_notify('artist_coins_mint_changed', JSON_BUILD_OBJECT('new', NEW.mint, 'old', OLD.mint)::TEXT);
    END IF;

    IF (OLD.damm_v2_pool IS NULL AND NEW.damm_v2_pool IS NOT NULL) 
        OR (OLD.damm_v2_pool IS NOT NULL AND NEW.damm_v2_pool IS NULL)
        OR OLD.damm_v2_pool != NEW.damm_v2_pool
    THEN
        PERFORM pg_notify('artist_coins_damm_v2_pool_changed', JSON_BUILD_OBJECT('new', NEW.damm_v2_pool, 'old', OLD.damm_v2_pool)::TEXT);
    END IF;
    
    RETURN NEW;
    EXCEPTION
        WHEN OTHERS THEN
            RAISE WARNING 'An error occurred in %: %', TG_NAME, SQLERRM;
            RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DO $$ 
BEGIN
    CREATE TRIGGER on_artist_coins_change
    AFTER INSERT OR UPDATE OR DELETE ON artist_coins
    FOR EACH ROW EXECUTE FUNCTION handle_artist_coins_change();
EXCEPTION
    WHEN others THEN NULL;  -- Ignore if trigger already exists
END $$;
COMMENT ON TRIGGER on_artist_coins_change ON artist_coins IS 'Notifies when artist coins are added, removed, or updated.'