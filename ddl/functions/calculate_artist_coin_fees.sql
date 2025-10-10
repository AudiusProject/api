BEGIN;
DROP FUNCTION IF EXISTS calculate_artist_coin_fees(TEXT);
CREATE OR REPLACE FUNCTION calculate_artist_coin_fees(artist_coin_mint TEXT)
RETURNS TABLE (
    unclaimed_dbc_fees NUMERIC,
    total_dbc_fees NUMERIC,
    unclaimed_damm_v2_fees NUMERIC,
    total_damm_v2_fees NUMERIC,
    unclaimed_fees NUMERIC,
    total_fees NUMERIC
) LANGUAGE sql AS $function$
    WITH
    damm_fees AS (
        SELECT
            pool.token_a_mint AS mint,
            (
                pool.fee_b_per_liquidity
                * (
                    position.unlocked_liquidity + position.vested_liquidity + position.permanent_locked_liquidity
                )
                / POWER (2, 128)
                + position.fee_b_pending
            ) AS total_damm_v2_fees,
            (
                (pool.fee_b_per_liquidity - position.fee_b_per_token_checkpoint)
                * (
                    position.unlocked_liquidity + position.vested_liquidity + position.permanent_locked_liquidity
                )
                / POWER (2, 128)
                + position.fee_b_pending
            ) AS unclaimed_damm_v2_fees
        FROM sol_meteora_damm_v2_pools pool
        JOIN sol_meteora_dbc_migrations migration ON migration.base_mint = pool.token_a_mint
        JOIN sol_meteora_damm_v2_positions position ON position.address = migration.first_position
        WHERE pool.token_a_mint = artist_coin_mint
    ),
    dbc_fees AS (
        SELECT
            base_mint AS mint,
            total_trading_quote_fee / 2 AS total_dbc_fees,
            creator_quote_fee / 2 AS unclaimed_dbc_fees
        FROM artist_coin_pools
        WHERE base_mint = artist_coin_mint
    )
    SELECT
        FLOOR(COALESCE(dbc_fees.unclaimed_dbc_fees, 0)) AS unclaimed_dbc_fees,
        FLOOR(COALESCE(dbc_fees.total_dbc_fees, 0)) AS total_dbc_fees,
        FLOOR(COALESCE(damm_fees.unclaimed_damm_v2_fees, 0)) AS unclaimed_damm_v2_fees,
        FLOOR(COALESCE(damm_fees.total_damm_v2_fees, 0)) AS total_damm_v2_fees,
        FLOOR(COALESCE(dbc_fees.unclaimed_dbc_fees, 0) + COALESCE(damm_fees.unclaimed_damm_v2_fees, 0)) AS unclaimed_fees,
        FLOOR(COALESCE(dbc_fees.total_dbc_fees, 0) + COALESCE(damm_fees.total_damm_v2_fees, 0)) AS total_fees
    FROM dbc_fees
    FULL OUTER JOIN damm_fees USING (mint);
$function$;
COMMIT;