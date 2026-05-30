CREATE OR REPLACE FUNCTION handle_associated_wallets()
RETURNS TRIGGER AS $$
DECLARE
    v_mint    varchar;
    v_user_id int;
    v_wallet  text;
    v_chain   text;
BEGIN
    IF TG_OP = 'DELETE' THEN
        v_user_id := OLD.user_id;
        v_wallet  := OLD.wallet;
        v_chain   := OLD.chain;
    ELSE
        v_user_id := NEW.user_id;
        v_wallet  := NEW.wallet;
        v_chain   := NEW.chain;
    END IF;

    -- Only act on INSERT, DELETE, or an is_delete flip on UPDATE — a no-op
    -- metadata UPDATE shouldn't recompute balances (preserves prior behavior).
    IF TG_OP = 'UPDATE' AND (NEW.is_delete IS NOT DISTINCT FROM OLD.is_delete) THEN
        RETURN NULL;
    END IF;

    -- wAUDIO / artist-coin (sol) balances: recompute every sol mint this wallet
    -- holds. For a chain=eth wallet this loop simply finds nothing.
    FOR v_mint IN
        SELECT DISTINCT mint FROM sol_token_account_balances WHERE owner = v_wallet
    LOOP
        PERFORM update_sol_user_balance_mint(v_user_id, v_mint);
    END LOOP;

    -- AUDIO (ETH-side) balance: linking / unlinking a chain=eth wallet changes
    -- the user's aggregated eth_user_balances total.
    IF v_chain = 'eth' THEN
        PERFORM update_eth_user_balance(v_user_id);
    END IF;

    RETURN NULL;
EXCEPTION
    WHEN OTHERS THEN
        RAISE WARNING 'An error occurred in %: %', TG_NAME, SQLERRM;
        RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
    CREATE TRIGGER on_associated_wallets
    AFTER INSERT OR UPDATE OR DELETE ON associated_wallets
    FOR EACH ROW EXECUTE PROCEDURE handle_associated_wallets();
EXCEPTION
  WHEN others THEN NULL;
END $$;
COMMENT ON TRIGGER on_associated_wallets ON associated_wallets IS 'Updates sol_user_balances and eth_user_balances when associated_wallets are added and removed';