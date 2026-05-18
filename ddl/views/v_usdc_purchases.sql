DROP VIEW IF EXISTS v_usdc_purchases;
CREATE VIEW v_usdc_purchases AS
SELECT
    sp.signature,
    sp.slot,
    sp.buyer_user_id,
    CASE sp.content_type
      WHEN 'track'    THEN t.owner_id
      WHEN 'album'    THEN p.playlist_owner_id
      WHEN 'playlist' THEN p.playlist_owner_id
    END AS seller_user_id,
    sp.amount,
    sp.content_type::usdc_purchase_content_type AS content_type,
    sp.content_id,
    sp.created_at,
    sp.created_at AS updated_at,
    GREATEST(
        sp.amount - COALESCE(
            CASE sp.content_type
              WHEN 'track' THEN (
                SELECT tph.total_price_cents * 10000
                  FROM track_price_history tph
                 WHERE tph.track_id = sp.content_id
                   AND tph.block_timestamp <= sp.created_at
                 ORDER BY tph.block_timestamp DESC
                 LIMIT 1
              )
              ELSE (
                SELECT aph.total_price_cents * 10000
                  FROM album_price_history aph
                 WHERE aph.playlist_id = sp.content_id
                   AND aph.block_timestamp <= sp.created_at
                 ORDER BY aph.block_timestamp DESC
                 LIMIT 1
              )
            END,
            sp.amount  -- no price_history match -> treat full amount as base price (extra_amount = 0)
        ),
        0
    ) AS extra_amount,
    sp.access_type::usdc_purchase_access_type AS access,
    sp.city, sp.region, sp.country,
    (
      SELECT COALESCE(
        jsonb_agg(
          jsonb_build_object(
            'user_id',       COALESCE(u_payout.user_id, u_sca.user_id),
            'payout_wallet', pay.to_account,
            'amount',        pay.amount,
            'percentage',    pay.amount * 100.0 / NULLIF(sp.amount, 0)
          )
          ORDER BY pay.route_index
        ),
        '[]'::jsonb
      )
        FROM sol_payments pay
        -- Historical match: which user had this Solana wallet set as their
        -- USDC payout wallet at the time of the purchase? Mirrors Python's
        -- add_wallet_info_to_splits, which joins UserPayoutWalletHistory
        -- filtered by block_timestamp < purchase_time.
        LEFT JOIN LATERAL (
          SELECT upwh.user_id
            FROM user_payout_wallet_history upwh
           WHERE upwh.spl_usdc_payout_wallet = pay.to_account
             AND upwh.block_timestamp <= sp.created_at
           ORDER BY upwh.block_timestamp DESC
           LIMIT 1
        ) u_payout ON TRUE
        -- Fallback: if the user never set a custom payout (so no history
        -- row exists), pay.to_account is their USDC user-bank PDA, which
        -- is stable over time and resolves via sol_claimable_accounts.
        LEFT JOIN sol_claimable_accounts sca
          ON sca.account = pay.to_account
         AND sca.mint = 'EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v'
        LEFT JOIN users u_sca
          ON u_sca.wallet = sca.ethereum_address
         AND u_sca.is_current = TRUE
       WHERE pay.signature = sp.signature
         AND pay.instruction_index = sp.instruction_index
    ) AS splits
FROM sol_purchases sp
LEFT JOIN tracks t
  ON sp.content_type = 'track'
 AND t.track_id = sp.content_id
 AND t.is_current = TRUE
LEFT JOIN playlists p
  ON sp.content_type IN ('album', 'playlist')
 AND p.playlist_id = sp.content_id
 AND p.is_current = TRUE
WHERE sp.is_valid IS TRUE;

COMMENT ON VIEW v_usdc_purchases IS 'Compatibility view exposing sol_purchases + sol_payments in the column shape API routes used to read from usdc_purchases. seller_user_id is the current content owner (not snapshotted at purchase time). extra_amount is amount paid minus base price from price history. vendor is intentionally dropped.';
