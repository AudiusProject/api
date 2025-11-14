BEGIN;


ALTER TABLE reward_codes ADD COLUMN IF NOT EXISTS signature TEXT;
COMMENT ON COLUMN reward_codes.signature IS 'Signature used to generate the reward code';

COMMIT;