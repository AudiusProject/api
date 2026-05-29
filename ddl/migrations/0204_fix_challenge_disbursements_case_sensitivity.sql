-- Fix case-sensitivity bug in v_challenge_disbursements.
--
-- The Go Solana indexer stores recipient_eth_address as lowercase
-- (strings.ToLower in reward_manager.go:45), but users.wallet is an
-- EIP-55 checksummed address (mixed-case). The previous JOIN used a
-- case-sensitive equality check, which meant the join always failed
-- for any user whose wallet differs in case from the stored lowercase
-- value — i.e., virtually every user.
--
-- Effect of the bug:
--   - All disbursements in sol_reward_disbursements were invisible
--     through v_challenge_disbursements.
--   - The /v1/challenges/undisbursed endpoint (which LEFT JOINs against
--     this view) treated every completed challenge as unpaid.
--   - The trending-challenge-rewards bot retried all of them; Solana
--     rejected with "specifier already used". Users received their
--     rewards correctly on-chain, but the bot's Slack report showed
--     null slots for everything.
--
-- Fix: normalise the wallet side of the join to lowercase before
-- comparing, matching the format already stored in the indexer table.

BEGIN;

DROP VIEW IF EXISTS v_challenge_disbursements;
CREATE VIEW v_challenge_disbursements AS
    SELECT
        rd.challenge_id,
        rd.specifier,
        rd.amount::text AS amount,
        rd.signature,
        rd.slot,
        rd.created_at,
        users.user_id
    FROM sol_reward_disbursements rd
    JOIN users
        ON LOWER(users.wallet) = rd.recipient_eth_address
       AND users.is_current = TRUE;

COMMENT ON VIEW v_challenge_disbursements IS 'Compatibility view that exposes sol_reward_disbursements in the column shape the API routes used to read from challenge_disbursements. Resolves user_id via the indexer-populated recipient_eth_address (see migration 0172). Join uses LOWER(users.wallet) because the Go indexer stores recipient_eth_address as lowercase.';

COMMIT;
