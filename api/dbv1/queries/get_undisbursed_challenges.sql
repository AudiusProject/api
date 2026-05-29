-- name: GetUndisbursedChallenges :many
SELECT
    users.handle,
    users.wallet,
    user_challenges.challenge_id,
    user_challenges.specifier,
    user_challenges.amount
FROM user_challenges
JOIN users ON users.user_id = user_challenges.user_id
-- Match raw disbursement rows by (challenge_id, specifier): a reward is disbursed
-- once per specifier on-chain regardless of recipient, and reading
-- sol_reward_disbursements directly avoids v_challenge_disbursements dropping
-- disbursements whose recipient wallet does not resolve to a current user.
LEFT JOIN sol_reward_disbursements AS challenge_disbursements
    ON challenge_disbursements.challenge_id = user_challenges.challenge_id
    AND challenge_disbursements.specifier = user_challenges.specifier
WHERE
    challenge_disbursements.challenge_id IS NULL
    AND user_challenges.is_complete
    AND user_challenges.user_id = @user_id
    AND (@challenge_id::text = '' OR user_challenges.challenge_id = @challenge_id)
    AND (@specifier::text = '' OR user_challenges.specifier = @specifier)
;