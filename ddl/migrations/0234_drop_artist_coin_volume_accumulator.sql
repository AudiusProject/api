-- Drop the artist coin volume accumulator. The all-time volume stat it fed was
-- removed from the product (unverifiable vendor figure; no other aggregator even
-- reports it), and CoinStatsOnchainJob no longer computes volume.
BEGIN;

DROP TABLE IF EXISTS artist_coin_volume_accumulator;

COMMIT;
