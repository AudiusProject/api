BEGIN;

CREATE TABLE IF NOT EXISTS sol_reward_manager_inits (
    signature TEXT NOT NULL,
    instruction_index INT NOT NULL,
    slot BIGINT NOT NULL,
    min_votes INT NOT NULL,
    reward_manager_state TEXT NOT NULL,
    token_source TEXT NOT NULL,
    mint TEXT NOT NULL,
    manager TEXT NOT NULL,
    authority TEXT NOT NULL,
    PRIMARY KEY (signature, instruction_index)
);
COMMENT ON TABLE sol_reward_manager_inits IS 'Stores Init instructions for the Reward Manager program';
COMMENT ON COLUMN sol_reward_manager_inits.min_votes IS 'Minimum number of votes required for reward distribution';
COMMENT ON COLUMN sol_reward_manager_inits.reward_manager_state IS 'Public key of the Reward Manager state account';
COMMENT ON COLUMN sol_reward_manager_inits.token_source IS 'Public key of the token source account (Note: Any token account on the authority account is valid)';
COMMENT ON COLUMN sol_reward_manager_inits.mint IS 'Public key of the mint for the token source account';
COMMENT ON COLUMN sol_reward_manager_inits.manager IS 'Public key of the manager account that initially has authority to create and remove senders without quorum';
COMMENT ON COLUMN sol_reward_manager_inits.authority IS 'Public key of the authority account, which holds the token accounts that reward manager can disburse from';

CREATE INDEX IF NOT EXISTS idx_sol_reward_manager_inits_mint ON sol_reward_manager_inits (mint);
COMMENT ON INDEX idx_sol_reward_manager_inits_mint IS 'Index to quickly find reward manager init instructions by mint of its token rewards';

COMMIT;