BEGIN;
SET LOCAL statement_timeout = 0;

-- Parity with the legacy usdc_purchases.created_at column. The Go indexer
-- writes new rows close to on-chain time, so DEFAULT NOW() is acceptable; the
-- backfill below corrects rows that came from the legacy table.
ALTER TABLE sol_purchases
    ADD COLUMN IF NOT EXISTS created_at TIMESTAMP DEFAULT NOW();

-- Backfill historical purchases that predate the Go indexer. from_account is
-- resolved via the buyer's USDC user_bank so the NOT NULL column has a real
-- value. Falls back to '' for buyers whose bank account is unknown — the
-- sol_purchases_from_account_idx tolerates empty strings.
INSERT INTO sol_purchases (
    signature, instruction_index, amount, slot,
    from_account, content_type, content_id, buyer_user_id,
    access_type, valid_after_blocknumber, is_valid,
    city, region, country, created_at
)
SELECT
    up.signature,
    0 AS instruction_index,
    up.amount,
    up.slot,
    COALESCE(uuba.bank_account, '') AS from_account,
    up.content_type::text,
    up.content_id,
    up.buyer_user_id,
    up.access::text,
    0 AS valid_after_blocknumber,
    TRUE AS is_valid,
    up.city, up.region, up.country,
    up.created_at
FROM usdc_purchases up
LEFT JOIN users u
    ON u.user_id = up.buyer_user_id AND u.is_current = TRUE
LEFT JOIN usdc_user_bank_accounts uuba
    ON uuba.ethereum_address = u.wallet
ON CONFLICT (signature, instruction_index) DO NOTHING;

-- Correct created_at for rows the Go indexer wrote before this migration ran:
-- those rows got NOW() from the column default, but the legacy table has the
-- real on-chain time. Only updates rows whose existing created_at is later
-- than the legacy value, so it leaves accurate Go-indexer writes alone.
UPDATE sol_purchases sp
   SET created_at = up.created_at
  FROM usdc_purchases up
 WHERE up.signature = sp.signature
   AND up.created_at < sp.created_at;

-- Explode legacy usdc_purchases.splits JSONB into sol_payments rows. The
-- element shape is {payout_wallet, amount, percentage, user_id, eth_wallet}
-- per add_wallet_info_to_splits() in
-- discovery-provider/src/queries/get_extended_purchase_gate.py.
INSERT INTO sol_payments (signature, instruction_index, route_index, to_account, amount, slot)
SELECT
    up.signature,
    0 AS instruction_index,
    (ord - 1)::int AS route_index,
    elem->>'payout_wallet' AS to_account,
    (elem->>'amount')::bigint AS amount,
    up.slot
FROM usdc_purchases up
CROSS JOIN LATERAL jsonb_array_elements(up.splits) WITH ORDINALITY arr(elem, ord)
WHERE elem->>'payout_wallet' IS NOT NULL
ON CONFLICT (signature, instruction_index, route_index) DO NOTHING;

-- Default sort across the purchases / sales / library routes is by created_at;
-- restore the index parity the legacy table had via idx_usdc_purchases_created_at.
CREATE INDEX IF NOT EXISTS sol_purchases_created_at_idx
    ON sol_purchases (created_at);

COMMIT;
