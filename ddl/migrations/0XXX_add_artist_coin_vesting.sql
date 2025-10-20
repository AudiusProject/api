-- Add vesting information to artist_coin_pools table
-- This tracks the reserved coins that unlock over 5 years after graduation

ALTER TABLE artist_coin_pools
ADD COLUMN IF NOT EXISTS reserved_amount NUMERIC DEFAULT 0,
ADD COLUMN IF NOT EXISTS claimed_vested_amount NUMERIC DEFAULT 0,
ADD COLUMN IF NOT EXISTS total_unlocked_amount NUMERIC DEFAULT 0,
ADD COLUMN IF NOT EXISTS graduation_date TIMESTAMP;

COMMENT ON COLUMN artist_coin_pools.reserved_amount IS 'Total amount of coins reserved for the artist that will unlock over 5 years';
COMMENT ON COLUMN artist_coin_pools.claimed_vested_amount IS 'Amount of unlocked coins that have already been claimed by the artist';
COMMENT ON COLUMN artist_coin_pools.total_unlocked_amount IS 'Total amount that has unlocked since graduation (increases daily over 5 years)';
COMMENT ON COLUMN artist_coin_pools.graduation_date IS 'Timestamp when the coin graduated (is_migrated became true)';

-- The claimable amount is calculated as: total_unlocked_amount - claimed_vested_amount
-- The locked amount is calculated as: reserved_amount - total_unlocked_amount

