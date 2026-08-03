-- Running all-time trading-volume accumulator per artist coin, replacing Birdeye's
-- all-time stats. Each CoinStatsOnchainJob run adds only trades in new slots
-- (slot > last_processed_slot), valued in USD at that interval's AUDIO price, so
-- historical USD isn't repriced at today's rate. total_volume is AUDIO-denominated.
BEGIN;

CREATE TABLE IF NOT EXISTS artist_coin_volume_accumulator (
    mint                 TEXT PRIMARY KEY,
    last_processed_slot  BIGINT NOT NULL DEFAULT 0,
    total_volume         DOUBLE PRECISION NOT NULL DEFAULT 0,
    total_volume_usd     DOUBLE PRECISION NOT NULL DEFAULT 0,
    updated_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE artist_coin_volume_accumulator IS
    'Running per-coin trading volume (AUDIO + USD) accumulated from pool quote-vault balance changes, watermarked by last_processed_slot. Written by CoinStatsOnchainJob.';

COMMIT;
