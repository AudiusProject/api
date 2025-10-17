BEGIN;

ALTER TABLE sol_reward_disbursements
ADD COLUMN IF NOT EXISTS recipient_eth_address TEXT DEFAULT '';
COMMENT ON COLUMN sol_reward_disbursements.recipient_eth_address IS 'The Ethereum address of the recipient of the reward.';

UPDATE sol_reward_disbursements
SET recipient_eth_address = users.wallet
FROM users
JOIN sol_claimable_accounts 
    ON sol_claimable_accounts.ethereum_address = users.wallet
WHERE sol_claimable_accounts.account = sol_reward_disbursements.user_bank
    AND sol_reward_disbursements.recipient_eth_address = '';

ALTER TABLE sol_reward_disbursements
ALTER COLUMN recipient_eth_address DROP DEFAULT;

COMMIT;