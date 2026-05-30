-- Keeps eth_user_balances fresh when the eth-indexer writes a wallet balance.
-- A single eth_wallet_balances row can back a user's primary wallet and/or any
-- number of users' linked chain=eth associated_wallets, so we recompute every
-- user the changed wallet maps to. NEW.wallet is already lowercase hex (the
-- eth_wallet_balances PK); users.wallet is lowered to match.
CREATE OR REPLACE FUNCTION handle_eth_wallet_balance_change()
RETURNS TRIGGER AS $$
DECLARE
    v_user_id int;
BEGIN
    -- Skip metadata-only updates (e.g. an updated_at touch with no balance
    -- delta) so we don't churn eth_user_balances for no reason.
    IF TG_OP = 'UPDATE' AND NEW.balance IS NOT DISTINCT FROM OLD.balance THEN
        RETURN NULL;
    END IF;

    FOR v_user_id IN
        SELECT user_id
          FROM users
         WHERE LOWER(wallet) = NEW.wallet
           AND is_current = TRUE

        UNION

        SELECT user_id
          FROM associated_wallets
         WHERE LOWER(wallet) = NEW.wallet
           AND chain = 'eth'
           AND is_current = TRUE
           AND is_delete = FALSE
    LOOP
        PERFORM update_eth_user_balance(v_user_id);
    END LOOP;

    RETURN NULL;
EXCEPTION
    WHEN OTHERS THEN
        RAISE WARNING 'An error occurred in %: %', TG_NAME, SQLERRM;
        RETURN NULL;
END;
$$ LANGUAGE plpgsql;

DO $$
BEGIN
    CREATE TRIGGER on_eth_wallet_balance_changes
    AFTER INSERT OR UPDATE ON eth_wallet_balances
    FOR EACH ROW EXECUTE PROCEDURE handle_eth_wallet_balance_change();
EXCEPTION
  WHEN others THEN NULL;
END $$;
COMMENT ON TRIGGER on_eth_wallet_balance_changes ON eth_wallet_balances IS
    'Recomputes eth_user_balances for affected users whenever an eth wallet balance changes.';
