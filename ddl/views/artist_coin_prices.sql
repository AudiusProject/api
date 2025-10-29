DROP VIEW IF EXISTS artist_coin_prices;
CREATE VIEW artist_coin_prices AS
    WITH dbc AS (
        SELECT
            artist_coins.mint,
            price_from_sqrt_price(dbc_pool.sqrt_price, artist_coins.decimals) 
                * dbc_quote_token.price AS price
        FROM artist_coins
        JOIN sol_meteora_dbc_pools dbc_pool
            ON dbc_pool.base_mint = artist_coins.mint
            AND dbc_pool.is_migrated = 0
        JOIN sol_meteora_dbc_configs dbc_config
            ON dbc_config.account = dbc_pool.config
        JOIN artist_coin_stats dbc_quote_token
            ON dbc_quote_token.mint = dbc_config.quote_mint
        WHERE is_migrated = 0
    ), damm_v2 AS (
        SELECT
            artist_coins.mint,
            price_from_sqrt_price(damm_v2_pool.sqrt_price, artist_coins.decimals) 
                * damm_v2_quote_token.price AS price
        FROM artist_coins
        JOIN sol_meteora_damm_v2_pools damm_v2_pool
            ON damm_v2_pool.token_a_mint = artist_coins.mint
        JOIN artist_coin_stats damm_v2_quote_token
            ON damm_v2_quote_token.mint = damm_v2_pool.token_b_mint
    )
    SELECT
        artist_coins.mint,
        damm_v2.price AS damm_v2_price,
        dbc.price AS dbc_price,
        stats.price AS stats_price,
        COALESCE(
            damm_v2.price,
            dbc.price,
            stats.price
        ) AS price
    FROM artist_coins
    LEFT JOIN dbc ON artist_coins.mint = dbc.mint
    LEFT JOIN damm_v2 ON artist_coins.mint = damm_v2.mint
    JOIN artist_coin_stats AS stats 
        ON stats.mint = artist_coins.mint;

COMMENT ON VIEW artist_coin_prices IS 'View that provides artist coin prices using DAMM V2 pool if available, DBC pools if not and still applicable, and stats table if nothing else. Makes use of the price of the quote token (AUDIO) from Birdeye if using a pool.';