BEGIN;

CREATE TABLE IF NOT EXISTS eth_wallet_balances (
    wallet TEXT PRIMARY KEY,
    balance NUMERIC NOT NULL DEFAULT 0,
    blocknumber BIGINT,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE eth_wallet_balances IS 'AUDIO ERC-20 balances (in wei) for tracked Ethereum wallets — primary users.wallet and chain=eth associated_wallets. Maintained event-driven by the eth-indexer (WebSocket subscription to the AUDIO Transfer topic, targeted balanceOf reads).';

CREATE INDEX IF NOT EXISTS eth_wallet_balances_updated_at_idx ON eth_wallet_balances (updated_at);
COMMENT ON INDEX eth_wallet_balances_updated_at_idx IS 'Supports staleness queries / catch-up sweeps.';

CREATE TABLE IF NOT EXISTS eth_indexer_checkpoints (
    name TEXT PRIMARY KEY,
    last_block BIGINT NOT NULL,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
COMMENT ON TABLE eth_indexer_checkpoints IS 'Resumable backfill checkpoints for the eth-indexer (last block whose Transfer events have been processed, keyed by subscription name).';

COMMIT;
