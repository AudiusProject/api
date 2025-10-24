CREATE OR REPLACE FUNCTION handle_dbc_pool()
RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('dbc_pools_changed', JSON_BUILD_OBJECT('new', NEW.account)::TEXT);
    RETURN NEW;
    EXCEPTION
        WHEN OTHERS THEN
            RAISE WARNING 'An error occurred in %: %', TG_NAME, SQLERRM;
            RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DO $$ 
BEGIN
    CREATE TRIGGER on_dbc_pool_change
    AFTER INSERT ON sol_meteora_dbc_pools
    FOR EACH ROW EXECUTE FUNCTION handle_dbc_pool();
EXCEPTION
    WHEN others THEN NULL;  -- Ignore if trigger already exists
END $$;
COMMENT ON TRIGGER on_dbc_pool_change ON sol_meteora_dbc_pools IS 'Notifies when DBC pools are added, removed, or updated.'