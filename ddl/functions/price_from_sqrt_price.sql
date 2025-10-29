BEGIN;
DROP FUNCTION IF EXISTS price_from_sqrt_price(NUMERIC, INT, INT);
CREATE OR REPLACE FUNCTION price_from_sqrt_price(sqrt_price NUMERIC, base_decimals INT, quote_decimals INT = 8)
RETURNS NUMERIC LANGUAGE sql AS $function$
    SELECT
        -- See: https://github.com/MeteoraAg/dynamic-bonding-curve-sdk/blob/a5519bf920935a9438fa200e067fffc6c6e40e27/packages/dynamic-bonding-curve/src/helpers/common.ts#L198
        ((sqrt_price * sqrt_price) * POWER(10, base_decimals - quote_decimals)) / POWER(2, 128) AS price
$function$;
COMMIT;