-- name: GetUsers :many
SELECT
  album_count,
  artist_pick_track_id,
  bio,
  follower_count,
  following_count as followee_count,
  handle,
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
  -- profile_picture todo
  repost_count,
  track_count,
  is_deactivated,
  is_available,
  -- erc_wallet,
  -- spl_wallet,
  -- spl_usdc_payout_wallet,
  supporter_count,
  supporting_count,
  -- total_audio_balance,
  wallet,
  balance,
  associated_wallets_balance,
  -- total_balance,
  -- waudio_balance,
  associated_sol_wallets_balance,
  blocknumber,
  u.created_at,
  is_storage_v2,
  creator_node_endpoint,
  -- current_user_followee_follow_count,  TODO: kill this



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
  -- cover_photo_cids,
  -- cover_photo_legacy,
  profile_picture_sizes,
  -- profile_picture_cids,
  -- profile_picture_legacy,
  has_collectibles,
  playlist_library,
  allow_ai_attribution

FROM users u
JOIN aggregate_user using (user_id)
LEFT JOIN user_balances using (user_id)
WHERE is_deactivated = false
  AND (
    handle_lc = lower(@handle)
    OR u.user_id = ANY(@ids::int[])
  )
ORDER BY u.user_id
;
