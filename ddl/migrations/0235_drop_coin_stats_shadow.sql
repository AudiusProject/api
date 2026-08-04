-- Retire the on-chain shadow of artist_coin_stats. CoinStatsOnchainJob now writes
-- artist_coin_stats directly (replacing the Birdeye-backed CoinStatsJob), so the
-- validation shadow table and its comparison view are no longer needed.
-- The comparison view depends on the table, so drop it first.
BEGIN;

DROP VIEW IF EXISTS artist_coin_stats_comparison;
DROP TABLE IF EXISTS artist_coin_stats_onchain;

COMMIT;
