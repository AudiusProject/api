CREATE OR REPLACE FUNCTION handle_meteora_dbc_migrations()
RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('meteora_dbc_migration', json_build_object('operation', TG_OP)::text);
    RETURN NEW;
    EXCEPTION
        WHEN OTHERS THEN
            RAISE WARNING 'An error occurred in %: %', TG_NAME, SQLERRM;
            RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DO $$ 
BEGIN
    CREATE TRIGGER on_meteora_dbc_migrations
    AFTER INSERT OR DELETE ON sol_meteora_dbc_migrations
    FOR EACH ROW EXECUTE FUNCTION handle_meteora_dbc_migrations();
EXCEPTION
    WHEN others THEN NULL;  -- Ignore if trigger already exists
END $$;
COMMENT ON TRIGGER on_meteora_dbc_migrations ON sol_meteora_dbc_migrations IS 'Notifies when a DBC pool migrates to a DAMM V2 pool.'