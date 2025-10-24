BEGIN;

CREATE TABLE IF NOT EXISTS sol_locker_vesting_escrows (
    account TEXT PRIMARY KEY,
    slot BIGINT NOT NULL,
    recipient TEXT NOT NULL,
    token_mint TEXT NOT NULL,
    creator TEXT NOT NULL,
    base TEXT NOT NULL,
    escrow_bump SMALLINT NOT NULL,
    update_recipient_mode SMALLINT NOT NULL,
    cancel_mode SMALLINT NOT NULL,
    token_program_flag SMALLINT NOT NULL,
    cliff_time BIGINT NOT NULL,
    frequency BIGINT NOT NULL,
    cliff_unlock_amount BIGINT NOT NULL,
    amount_per_period BIGINT NOT NULL,
    number_of_period BIGINT NOT NULL,
    total_claimed_amount BIGINT NOT NULL,
    vesting_start_time BIGINT NOT NULL,
    cancelled_at BIGINT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

COMMIT;