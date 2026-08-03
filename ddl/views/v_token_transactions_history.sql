DROP VIEW IF EXISTS v_token_transactions_history;
CREATE VIEW v_token_transactions_history AS
SELECT
    bc.signature,
    bc.mint,
    bc.account AS user_bank,
    users.user_id,
    bc.block_timestamp AS transaction_date,
    bc.created_at,
    bc.slot,
    -- Legacy semantic: `change` is the unsigned amount, direction expressed via
    -- `method` ('send'/'receive'). sol_token_account_balance_changes stores
    -- signed deltas; preserve that as the source of truth for direction.
    ABS(bc.change)::text AS change,
    bc.balance::text AS balance,
    CASE WHEN bc.change < 0 THEN 'send' ELSE 'receive' END AS method,
    CASE
      -- Reward disbursement to this user_bank (USER_REWARD or TRENDING_REWARD).
      WHEN rd.signature IS NOT NULL THEN
        CASE WHEN c.type = 'trending' THEN 'trending_reward' ELSE 'user_reward' END
      -- Content purchase via Payment Router (USDC). Checked before the
      -- recover_withdrawal branch because on chain they're mutually exclusive;
      -- if both somehow appeared a content purchase is the higher-fidelity label.
      WHEN p.signature IS NOT NULL THEN 'purchase_content'
      -- Memo-tagged transfer types. sol_transfer_memo_types is populated by
      -- the program indexer when a recognized memo (or, for prepare_withdrawal,
      -- the Jupiter program) accompanies the claimable transfer / route.
      WHEN tmt.memo_type IS NOT NULL THEN tmt.memo_type
      -- Claimable Tokens transfer (in or out). Tips are claimable transfers where
      -- both endpoints resolve to distinct Audius users; everything else is a
      -- bare transfer.
      WHEN cat.signature IS NOT NULL
       AND from_owner.user_id IS NOT NULL
       AND to_owner.user_id IS NOT NULL
       AND from_owner.user_id <> to_owner.user_id THEN 'tip'
      WHEN cat.signature IS NOT NULL THEN 'transfer'
      -- Anything else (Stripe/Coinbase top-ups on wAUDIO, untyped balance
      -- changes) degrades to transfer until vendor-memo capture lands on the
      -- Go indexer.
      ELSE 'transfer'
    END AS transaction_type,
    -- Match legacy audio_transactions_history.tx_metadata format:
    --   tips: str(counterpart_user_id)
    --   rewards: challenge_id
    --   withdrawal: the on-chain destination account (legacy stored the
    --     resolved wallet; the Go indexer surfaces cat.to_account as the
    --     closest equivalent — resolved-wallet capture is a follow-up).
    --   else: null
    CASE
      WHEN rd.signature IS NOT NULL THEN rd.challenge_id
      WHEN tmt.memo_type = 'withdrawal' AND cat.to_account IS NOT NULL
        THEN cat.to_account
      WHEN cat.signature IS NOT NULL AND bc.change > 0 AND from_owner.user_id IS NOT NULL
        THEN from_owner.user_id::text
      WHEN cat.signature IS NOT NULL AND bc.change < 0 AND to_owner.user_id IS NOT NULL
        THEN to_owner.user_id::text
      ELSE NULL
    END AS tx_metadata
FROM sol_token_account_balance_changes bc
JOIN sol_claimable_accounts sca
    ON sca.account = bc.account
   AND sca.mint = bc.mint
LEFT JOIN users
    ON users.wallet = sca.ethereum_address
   AND users.is_current = TRUE
LEFT JOIN sol_claimable_account_transfers cat
    ON cat.signature = bc.signature
   AND (cat.from_account = bc.account OR cat.to_account = bc.account)
LEFT JOIN sol_claimable_accounts from_sca
    ON from_sca.account = cat.from_account
   AND from_sca.mint = bc.mint
LEFT JOIN users from_owner
    ON from_owner.wallet = from_sca.ethereum_address
   AND from_owner.is_current = TRUE
LEFT JOIN sol_claimable_accounts to_sca
    ON to_sca.account = cat.to_account
   AND to_sca.mint = bc.mint
LEFT JOIN users to_owner
    ON to_owner.wallet = to_sca.ethereum_address
   AND to_owner.is_current = TRUE
LEFT JOIN sol_reward_disbursements rd
    ON rd.signature = bc.signature
   AND rd.user_bank = bc.account
LEFT JOIN challenges c
    ON c.id = rd.challenge_id
LEFT JOIN sol_purchases p
    ON p.signature = bc.signature
   AND p.from_account = bc.account
LEFT JOIN sol_transfer_memo_types tmt
    ON tmt.signature = bc.signature;

COMMENT ON VIEW v_token_transactions_history IS 'Mint-agnostic transactions history derived from sol_token_account_balance_changes (the hub: only table with both mint and block_timestamp). Per-row transaction_type derived by LEFT JOIN to typed tables (sol_claimable_account_transfers, sol_reward_disbursements, sol_purchases, sol_transfer_memo_types). Callers filter by mint at query time. Vendor purchase types (PURCHASE_STRIPE/COINBASE/UNKNOWN) on AUDIO still degrade to bare transfer until the AUDIO mint subscription + vendor-memo capture land.';
