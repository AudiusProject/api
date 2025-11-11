BEGIN;
DROP FUNCTION IF EXISTS calculate_artist_coin_fee_earnings(TEXT);
CREATE OR REPLACE FUNCTION calculate_artist_coin_fee_earnings(artist_coin_mint TEXT)
RETURNS TABLE (
    unclaimed_fees NUMERIC,
    total_fees NUMERIC
) LANGUAGE sql AS $function$
    WITH
    artist_positions AS (
        SELECT
            first_position AS position,
            damm_v2_pool AS pool,
            base_mint AS mint
        FROM sol_meteora_dbc_migrations
        WHERE base_mint = artist_coin_mint

        UNION ALL

        SELECT
            position,
            pool,
            token_a_mint AS mint
        FROM sol_meteora_damm_v2_initialize_custom_pool_instructions
        WHERE token_a_mint = artist_coin_mint
    ),
    damm_v2_fees AS (
        -- fee = totalLiquidity * feePerTokenStore
        -- precision: (totalLiquidity * feePerTokenStore) >> 128
        -- See: https://github.com/MeteoraAg/damm-v2-sdk/blob/70d1af59689039a1dc700dee8f741db48024d02d/src/helpers/utils.ts#L190-L191
        SELECT
            pool.token_a_mint AS mint,
            (
                pool.fee_b_per_liquidity
                * (
                    position.unlocked_liquidity + position.vested_liquidity + position.permanent_locked_liquidity
                )
                / POWER (2, 128)
                + position.fee_b_pending
            ) AS total,
            (
                (pool.fee_b_per_liquidity - position.fee_b_per_token_checkpoint)
                * (
                    position.unlocked_liquidity + position.vested_liquidity + position.permanent_locked_liquidity
                )
                / POWER (2, 128)
                + position.fee_b_pending
            ) AS unclaimed
        FROM sol_meteora_damm_v2_pools pool
        JOIN artist_positions p ON pool.account = p.pool
        JOIN sol_meteora_damm_v2_positions position ON p.position = position.account
        WHERE pool.token_a_mint = artist_coin_mint
    ),
    dbc_fees AS (
        SELECT
            base_mint AS mint,
            total_trading_quote_fee / 2 AS total,
            creator_quote_fee AS unclaimed
        FROM artist_coin_pools
        WHERE base_mint = artist_coin_mint
    ),
    all_fees AS (
        SELECT * FROM damm_v2_fees
        UNION ALL
        SELECT * FROM dbc_fees
    )
    SELECT
        COALESCE(FLOOR(SUM(unclaimed)), 0) AS unclaimed_fees,
        COALESCE(FLOOR(SUM(total)), 0) AS total_fees
    FROM artist_coins
    LEFT JOIN all_fees USING (mint)
    WHERE artist_coins.mint = artist_coin_mint;
$function$;
COMMIT;