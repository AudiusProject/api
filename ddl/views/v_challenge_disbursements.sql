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
        ON users.wallet = rd.recipient_eth_address
       AND users.is_current = TRUE;

COMMENT ON VIEW v_challenge_disbursements IS 'Compatibility view that exposes sol_reward_disbursements in the column shape the API routes used to read from challenge_disbursements. Resolves user_id via the indexer-populated recipient_eth_address (see migration 0172).';
