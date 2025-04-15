-- name: GetUndisbursedChallenges :many
SELECT
    users.handle,
    users.wallet,
    user_challenges.challenge_id,
    user_challenges.specifier
FROM user_challenges
JOIN users ON users.user_id = user_challenges.user_id
LEFT JOIN challenge_disbursements
    ON challenge_disbursements.challenge_id = user_challenges.challenge_id
    AND challenge_disbursements.specifier = user_challenges.specifier
WHERE
    challenge_disbursements.challenge_id IS NULL
    AND user_challenges.is_complete
    AND user_challenges.user_id = @user_id
    AND (@challenge_id::text = '' OR user_challenges.challenge_id = @challenge_id)
    AND (@specifier::text = '' OR user_challenges.specifier = @specifier)
;