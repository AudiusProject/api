-- Hourly USD price snapshots per artist coin, used to compute the 24h price change
-- (history_24h_price / price_change_24h_percent) on-chain, replacing Birdeye.
-- Written by CoinStatsOnchainJob; kept small via hourly binning + 7-day retention.
BEGIN;

CREATE TABLE IF NOT EXISTS artist_coin_price_history (
    mint       TEXT NOT NULL,
    timestamp  TIMESTAMP NOT NULL,
    price      DOUBLE PRECISION NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (mint, timestamp)
);

CREATE INDEX IF NOT EXISTS artist_coin_price_history_mint_ts_idx
    ON artist_coin_price_history (mint, timestamp DESC);

COMMENT ON TABLE artist_coin_price_history IS
    'Hourly USD price snapshots per artist coin, used to compute 24h price change.';

COMMIT;
