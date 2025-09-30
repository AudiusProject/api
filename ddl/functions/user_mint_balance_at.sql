-- Gets the aggregate balance for a user/mint combo at a given timestamp using sol_token_account_balance_changes
-- It combines changes for accounts owned by any associated_wallets and sol_claimable_accounts addresses
-- that map to the user.
CREATE OR REPLACE FUNCTION user_mint_balance_at(
  p_user_id  integer,
  p_mint     text,
  p_cutoff   timestamptz
) RETURNS bigint
LANGUAGE sql
STABLE PARALLEL SAFE
AS $$
WITH owners AS (
  SELECT aw.wallet AS owner_address
  FROM associated_wallets aw
  WHERE aw.user_id = p_user_id
    AND aw.chain = 'sol'
    AND aw.is_delete = FALSE
),
accounts AS (
  SELECT sca.account
  FROM users u
  JOIN sol_claimable_accounts sca
    ON sca.ethereum_address = u.wallet
  WHERE u.user_id = p_user_id
),
latest AS (
  SELECT DISTINCT ON (s.mint, s.account) s.balance
  FROM (
    SELECT stabc.*
    FROM sol_token_account_balance_changes stabc
    WHERE stabc.mint = p_mint
      AND stabc.block_timestamp <= p_cutoff
      AND stabc.account IN (SELECT account FROM accounts)

    UNION ALL

    SELECT stabc.*
    FROM sol_token_account_balance_changes stabc
    WHERE stabc.mint = p_mint
      AND stabc.block_timestamp <= p_cutoff
      AND stabc.owner IN (SELECT owner_address FROM owners)
  ) AS s
  ORDER BY s.mint, s.account, s.block_timestamp DESC, s.slot DESC
)
SELECT COALESCE(SUM(balance), 0)::bigint
FROM latest;
$$;