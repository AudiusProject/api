-- name: GetUsers :many
SELECT
  album_count,
  artist_pick_track_id,
  bio,

  -- todo: this can sometimes be a Qm cid
  -- sometiems be a json string...
  cover_photo,

  follower_count,
  following_count as followee_count,
  handle,
  'hashid' as id,
  u.user_id,
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

  -- todo: this can sometimes be a Qm cid
  -- sometiems be a json string...
  profile_picture,

  repost_count,
  track_count,
  is_deactivated,
  is_available,
  wallet as erc_wallet,
  user_bank_accounts.bank_account as spl_wallet,
  spl_usdc_payout_wallet,
  supporter_count,
  supporting_count,
  wallet,
  balance,
  associated_wallets_balance,

  -- total_balance
  (
    coalesce(balance, '0')::NUMERIC +
    coalesce(associated_wallets_balance, '0')::NUMERIC +
    -- to wei
    (coalesce(associated_sol_wallets_balance, '0')::NUMERIC * 10^10) +
    (coalesce(waudio, '0')::NUMERIC * 10^10)
  )::NUMERIC::TEXT AS total_balance,

  -- total_audio_balance,
  FLOOR(
    (
      coalesce(balance, '0')::NUMERIC +
      coalesce(associated_wallets_balance, '0')::NUMERIC +
      -- to wei
      (coalesce(associated_sol_wallets_balance, '0')::NUMERIC * 10^10) +
      (coalesce(waudio, '0')::NUMERIC * 10^10)
    ) / 1e18
  )::INT AS total_audio_balance,

  coalesce(waudio, '0') as waudio_balance,
  coalesce(associated_sol_wallets_balance, '0') as associated_sol_wallets_balance,
  blocknumber,
  u.created_at,
  is_storage_v2,
  creator_node_endpoint,
  10 as current_user_followee_follow_count,  -- TODO: either compute or remove this



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
  allow_ai_attribution

FROM users u
JOIN aggregate_user using (user_id)
LEFT JOIN user_balances using (user_id)
LEFT JOIN user_bank_accounts on u.wallet = user_bank_accounts.ethereum_address
WHERE u.user_id = ANY(@ids::int[])
ORDER BY u.user_id
;
