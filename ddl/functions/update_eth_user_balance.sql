-- Recompute one user's aggregated ETH-side AUDIO balance (wei) into
-- eth_user_balances. ETH analog of update_sol_user_balance_mint: sums
-- eth_wallet_balances across the user's primary wallet + linked chain=eth
-- associated_wallets. Always writes exactly one row (no GROUP BY) so unlinking
-- the last wallet correctly drives the balance to 0 rather than leaving a stale
-- value.
CREATE OR REPLACE FUNCTION update_eth_user_balance(p_user_id int)
RETURNS VOID AS $$
BEGIN
    INSERT INTO eth_user_balances (user_id, balance, updated_at, created_at)
    SELECT
        p_user_id,
        COALESCE(SUM(ewb.balance), 0),
        NOW(),
        NOW()
    FROM eth_wallet_balances ewb
    WHERE ewb.wallet IN (
        -- eth_wallet_balances PK is lowercase hex; users.wallet can be
        -- mixed-case, associated_wallets are canonical lowercase (0207).
        SELECT LOWER(wallet)
          FROM users
         WHERE user_id = p_user_id
           AND is_current = TRUE
           AND wallet IS NOT NULL

        UNION ALL

        SELECT LOWER(wallet)
          FROM associated_wallets
         WHERE user_id = p_user_id
           AND chain = 'eth'
           AND is_current = TRUE
           AND is_delete = FALSE
    )
    ON CONFLICT (user_id)
    DO UPDATE SET
        balance = EXCLUDED.balance,
        updated_at = NOW();
END;
$$ LANGUAGE plpgsql;
