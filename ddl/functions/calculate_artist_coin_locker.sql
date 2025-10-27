BEGIN;
DROP FUNCTION IF EXISTS calculate_artist_coin_locker(TEXT);
CREATE OR REPLACE FUNCTION calculate_artist_coin_locker(artist_coin_mint TEXT)
RETURNS TABLE (
    address TEXT,
    unlocked BIGINT,
    locked BIGINT,
    claimable BIGINT
) LANGUAGE sql AS $function$
    WITH escrow AS (
        SELECT
            account,
            amount_per_period,
            number_of_period,
            total_claimed_amount,
            cliff_unlock_amount,
            CASE WHEN to_timestamp(cliff_time) < NOW() THEN 0 ELSE cliff_unlock_amount END AS cliff_unlocked_amount,
            LEAST(
                FLOOR(
                    (EXTRACT(EPOCH FROM NOW()) - vesting_start_time) / frequency
                ),
                number_of_period
            ) AS periods_completed
        FROM sol_locker_vesting_escrows
        WHERE token_mint = artist_coin_mint
    )
    SELECT
        account AS address,
        cliff_unlocked_amount + periods_completed * amount_per_period AS unlocked,
        cliff_unlock_amount - cliff_unlocked_amount + (number_of_period - periods_completed) * amount_per_period AS locked,
        cliff_unlocked_amount + periods_completed * amount_per_period - total_claimed_amount AS claimable
    FROM escrow;
$function$;
COMMIT;