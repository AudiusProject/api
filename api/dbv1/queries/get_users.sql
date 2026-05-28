-- name: GetUsers :many
SELECT
  album_count,
  artist_pick_track_id,
  bio,

  -- todo: this can sometimes be a Qm cid
  -- sometimes be a json string...
  cover_photo,

  follower_count,
  following_count as followee_count,
  handle,
  u.user_id as id,
  u.user_id::int as user_id,
  is_verified,
  twitter_handle,
  instagram_handle,
  tiktok_handle,
  verified_with_twitter,
  verified_with_instagram,
  verified_with_tiktok,
  website,
  donation,
  location,
  name,
  playlist_count,
  profile_type,

  -- todo: this can sometimes be a Qm cid
  -- sometimes be a json string...
  profile_picture,

  repost_count,
  -- Use total_track_count when viewing own profile, otherwise use track_count
  (CASE
    WHEN u.user_id = @my_id::int THEN total_track_count
    ELSE track_count
  END)::bigint as track_count,
  is_deactivated,
  is_available,
  wallet as erc_wallet,
  user_bank_accounts.bank_account as spl_wallet,
  usdc_user_bank_accounts.bank_account as usdc_wallet,
  spl_usdc_payout_wallet,
  supporter_count,
  supporting_count,
  wallet,

  -- Legacy `balance` (ETH-side AUDIO in wei) and `associated_wallets_balance`
  -- (the split between primary and linked wallets) are collapsed into
  -- v_user_balances.eth_balance — sol_user_balances doesn't preserve a
  -- user_bank-vs-linked breakdown so we don't preserve one on the ETH side
  -- either. Surface the full ETH total under `balance` and zero out
  -- `associated_wallets_balance` so the response shape stays the same and
  -- `total_balance` still adds up to the right number.
  coalesce(eth_balance, '0') as balance,
  '0' as associated_wallets_balance,

  -- total_balance: eth_balance is in wei, sol_balance is in wAUDIO base units
  -- (8 decimals) so multiply by 10^10 to convert.
  (
    coalesce(eth_balance, '0')::NUMERIC +
    (coalesce(sol_balance, '0')::NUMERIC * 10^10)
  )::NUMERIC::TEXT AS total_balance,

  -- total_audio_balance,
  FLOOR(
    (
      coalesce(eth_balance, '0')::NUMERIC +
      (coalesce(sol_balance, '0')::NUMERIC * 10^10)
    ) / 1e18
  )::INT AS total_audio_balance,

  -- payout wallet
  coalesce(
    spl_usdc_payout_wallet,
    usdc_user_bank_accounts.bank_account,
    ''
  )::text as payout_wallet,

  -- Same collapse on the sol side: full sol total under `waudio_balance`,
  -- `associated_sol_wallets_balance` zeroed.
  coalesce(sol_balance, '0') as waudio_balance,
  '0' as associated_sol_wallets_balance,
  blocknumber,
  u.created_at,
  is_storage_v2,
  creator_node_endpoint,

  -- "Of the people I follow, how many also follow this user?"
  --
  -- Drive the loop from my followees (always small — a few thousand at
  -- most) and probe whether each follows the target user. The previous
  -- shape let Postgres pick a Merge Join that walked the full follower
  -- list of the target — for popular target users that's millions of
  -- rows. The LIMIT 1 OFFSET 0 inside the EXISTS is the same
  -- optimization fence used by the feed query: it pins the planner to
  -- nested-loop semantics so the plan never flips to merge join.
  (
    SELECT count(*)
    FROM follows mf
    WHERE @my_id > 0
      AND @my_id != u.user_id -- don't compute when viewing own profile
      AND mf.follower_user_id = @my_id
      AND mf.is_delete = false
      AND EXISTS (
        SELECT 1 FROM (
          SELECT 1 FROM follows f
          WHERE f.follower_user_id = mf.followee_user_id
            AND f.followee_user_id = u.user_id
            AND f.is_delete = false
          LIMIT 1 OFFSET 0
        ) x
      )
  ) AS current_user_followee_follow_count,

  (
    SELECT count(*) > 0
    FROM follows
    WHERE @my_id > 0
      AND follower_user_id = @my_id
      AND followee_user_id = u.user_id
      AND is_delete = false
  ) AS does_current_user_follow,

  (
    SELECT count(*) > 0
    FROM subscriptions
    WHERE @my_id > 0
      AND subscriber_id = @my_id
      AND user_id = u.user_id
      AND is_delete = false
  ) AS does_current_user_subscribe,

  (
    SELECT count(*) > 0
    FROM follows
    WHERE @my_id > 0
      AND followee_user_id = @my_id
      AND follower_user_id = u.user_id
      AND is_delete = false
  ) AS does_follow_current_user,

  handle_lc,
  u.updated_at,
  cover_photo_sizes,
  null as cover_photo_cids, -- todo: what goes in here?
  null as cover_photo_legacy, -- todo:

  profile_picture_sizes,
  null as profile_picture_cids, -- todo
  null as profile_picture_legacy, -- todo

  has_collectibles,
  allow_ai_attribution,
  coin_flair_mint,

  (
    SELECT JSON_BUILD_OBJECT(
      'mint', mint,
      'logo_uri', logo_uri,
      'banner_image_url', banner_image_url,
      'ticker', ticker
    )::jsonb
    FROM artist_coins
    WHERE artist_coins.mint =
      -- User explicitly disabled flair
      CASE WHEN (u.coin_flair_mint = '') THEN NULL
      ELSE COALESCE(
        -- Use preferred flair if valid artist coin and user has a balance
        CASE WHEN (u.coin_flair_mint IS NOT NULL) THEN
          (
            SELECT sol_user_balances.mint
            FROM sol_user_balances
            JOIN artist_coins ON artist_coins.mint = sol_user_balances.mint
            WHERE sol_user_balances.user_id = u.user_id
              AND sol_user_balances.balance > 0
              AND sol_user_balances.mint = u.coin_flair_mint
            LIMIT 1
          )
        ELSE NULL
        END,
        -- Owned first
        (
          SELECT artist_coins.mint
          FROM artist_coins
          WHERE artist_coins.user_id = u.user_id
          LIMIT 1
        ),
        -- Then highest USD value held
        (
          SELECT sol_user_balances.mint
          FROM sol_user_balances
          JOIN artist_coins ON artist_coins.mint = sol_user_balances.mint -- ensure mapped in artist_coins
          LEFT JOIN artist_coin_stats ON artist_coin_stats.mint = sol_user_balances.mint
          LEFT JOIN artist_coin_pools ON artist_coin_pools.base_mint = sol_user_balances.mint
          WHERE sol_user_balances.user_id = u.user_id
            AND sol_user_balances.balance > 0
            AND sol_user_balances.mint NOT IN (
              '9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM', -- ignore prod wAUDIO
              'BELGiMZQ34SDE6x2FUaML2UHDAgBLS64xvhXjX5tBBZo', -- ignore stage wAUDIO
              'EPjFWdd5AufqSSqeM2qN1xzybapC8G4wEGGkZwyTDt1v' -- ignore USDC
            )
          ORDER BY (sol_user_balances.balance * COALESCE(artist_coin_stats.price, artist_coin_pools.price_usd, 0)) / POWER(10, artist_coins.decimals) DESC
          LIMIT 1
        )
      )
    END
  ) AS artist_coin_badge

FROM users u
JOIN aggregate_user using (user_id)
LEFT JOIN v_user_balances using (user_id)
LEFT JOIN user_bank_accounts on u.wallet = user_bank_accounts.ethereum_address
LEFT JOIN usdc_user_bank_accounts on u.wallet = usdc_user_bank_accounts.ethereum_address
WHERE u.user_id = ANY(@ids::int[])
ORDER BY u.user_id
;
