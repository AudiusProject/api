-- Side-by-side comparison of the Birdeye-populated artist_coin_stats against the
-- on-chain-derived artist_coin_stats_onchain, for validating CoinStatsOnchainJob
-- before cutover. `*_pct_diff` = (onchain - birdeye) / birdeye * 100.
-- A large diff (or NULL onchain where birdeye has a value) flags a missed pool,
-- external market, or migration lag to investigate.
DROP VIEW IF EXISTS artist_coin_stats_comparison;
CREATE VIEW artist_coin_stats_comparison AS
    SELECT
        ac.mint,
        ac.ticker,

        b.price            AS birdeye_price,
        o.price            AS onchain_price,
        ROUND((((o.price - b.price) / NULLIF(b.price, 0)) * 100)::numeric, 2)                     AS price_pct_diff,

        b.market_cap       AS birdeye_market_cap,
        o.market_cap       AS onchain_market_cap,
        ROUND((((o.market_cap - b.market_cap) / NULLIF(b.market_cap, 0)) * 100)::numeric, 2)       AS market_cap_pct_diff,

        b.holder           AS birdeye_holder,
        o.holder           AS onchain_holder,
        (o.holder - b.holder)                                                                      AS holder_diff,

        b.liquidity        AS birdeye_liquidity,
        o.liquidity        AS onchain_liquidity,
        ROUND((((o.liquidity - b.liquidity) / NULLIF(b.liquidity, 0)) * 100)::numeric, 2)          AS liquidity_pct_diff,

        b.total_volume_usd AS birdeye_total_volume_usd,
        o.total_volume_usd AS onchain_total_volume_usd,
        ROUND((((o.total_volume_usd - b.total_volume_usd) / NULLIF(b.total_volume_usd, 0)) * 100)::numeric, 2) AS total_volume_usd_pct_diff,

        b.price_change_24h_percent AS birdeye_price_change_24h_percent,
        o.price_change_24h_percent AS onchain_price_change_24h_percent,

        o.updated_at       AS onchain_updated_at,
        b.updated_at       AS birdeye_updated_at
    FROM artist_coins ac
    LEFT JOIN artist_coin_stats b          ON b.mint = ac.mint
    LEFT JOIN artist_coin_stats_onchain o  ON o.mint = ac.mint;

COMMENT ON VIEW artist_coin_stats_comparison IS
    'Compares Birdeye artist_coin_stats vs on-chain artist_coin_stats_onchain per coin (values + % diff) to validate CoinStatsOnchainJob before cutover.';
