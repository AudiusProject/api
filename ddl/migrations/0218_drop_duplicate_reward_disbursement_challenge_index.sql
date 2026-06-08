-- sol_reward_disbursements_challenge_specifier_idx duplicates the checked-in
-- sol_reward_disbursements_challenge_idx definition on (challenge_id, specifier).
-- Keep the original index and remove the backfill-era duplicate.

drop index concurrently if exists sol_reward_disbursements_challenge_specifier_idx;
