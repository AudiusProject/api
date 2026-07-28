-- Shadow table for the on-chain-derived replacement of CoinStatsJob.
-- Mirrors artist_coin_stats exactly so the on-chain job can be validated against
-- the Birdeye-populated table before cutover (and so cutover is a repoint/rename
-- with no consumer-struct changes). Populated by CoinStatsOnchainJob.
BEGIN;

CREATE TABLE IF NOT EXISTS artist_coin_stats_onchain (
    LIKE artist_coin_stats INCLUDING ALL
);

COMMENT ON TABLE artist_coin_stats_onchain IS
    'On-chain-derived shadow of artist_coin_stats, written by CoinStatsOnchainJob. Used to validate against the Birdeye-populated artist_coin_stats before replacing it.';

COMMIT;
