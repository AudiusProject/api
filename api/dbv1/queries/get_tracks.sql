-- name: GetTracks :many
SELECT
  t.track_id,
  description,
  genre,
  'hashid' as id,
  track_cid,
  preview_cid,
  orig_file_cid,
  orig_filename,
  is_original_available,
  mood,
  release_date,
  repost_count,
  save_count as favorite_count,
  comment_count,
  tags,
  title,
  track_routes.slug as slug,
  duration,
  is_downloadable,
  aggregate_plays.count as play_count,
  ddex_app,
  -- playlists_containing_track,
  pinned_comment_id,
  -- album_backlink,
  t.blocknumber,
  create_date,
  t.created_at,
  cover_art_sizes,
  credits_splits,
  isrc,
  license,
  iswc,
  field_visibility,
  -- followee_reposts,

  (
    SELECT count(*) > 0
    FROM reposts
    WHERE @my_id > 0
      AND user_id = @my_id
      AND repost_type = 'track'
      AND repost_item_id = t.track_id
      AND is_delete = false
  ) AS has_current_user_reposted,

  (
    SELECT count(*) > 0
    FROM saves
    WHERE @my_id > 0
      AND user_id = @my_id
      AND save_type = 'track'
      AND save_item_id = t.track_id
      AND is_delete = false
  ) AS has_current_user_saved,

  is_scheduled_release,
  is_unlisted,

  (
    SELECT json_agg(
      json_build_object(
        'user_id', r.user_id::text,
        'repost_item_id', repost_item_id::text, -- this is redundant
        'repost_type', 'RepostType.track', -- some sqlalchemy bs
        'created_at', r.created_at -- this is not actually present in python response?
      )
    )
    FROM (
      SELECT user_id, repost_item_id, reposts.created_at
      FROM reposts
      JOIN follows ON followee_user_id = reposts.user_id AND follower_user_id = @my_id
      JOIN aggregate_user USING (user_id)
      WHERE repost_item_id = t.track_id
        AND repost_type = 'track'
        AND reposts.is_delete = false
      ORDER BY follower_count DESC
      LIMIT 6
    ) r
  )::jsonb as followee_reposts,

  (
    SELECT json_agg(
      json_build_object(
        'user_id', r.user_id::text,
        'favorite_item_id', r.save_item_id::text, -- this is redundant
        'favorite_type', 'SaveType.track', -- some sqlalchemy bs
        'created_at', r.created_at -- this is not actually present in python response?
      )
    )
    FROM (
      SELECT user_id, save_item_id, saves.created_at
      FROM saves
      JOIN follows ON followee_user_id = saves.user_id AND follower_user_id = @my_id
      JOIN aggregate_user USING (user_id)
      WHERE save_item_id = t.track_id
        AND save_type = 'track'
        AND saves.is_delete = false
      ORDER BY follower_count DESC
      LIMIT 6
    ) r
  )::jsonb as followee_favorites,

  (
    SELECT json_build_object(
      'tracks', json_agg(
        json_build_object(
          'has_remix_author_reposted', repost_item_id is not null,
          'has_remix_author_saved', save_item_id is not null,
          'parent_track_id', r.parent_track_id,
          'parent_user_id', r.parent_owner_id
        )
      )
    )
    FROM (
      SELECT
        track_id as parent_track_id,
        owner_id as parent_owner_id,
        repost_item_id,
        save_item_id
      FROM remixes
      JOIN tracks parent_track ON parent_track_id = parent_track.track_id AND child_track_id = t.track_id
      LEFT JOIN reposts ON repost_type = 'track' AND repost_item_id = t.track_id AND reposts.user_id = parent_track.owner_id AND reposts.is_delete = false
      LEFT JOIN saves ON save_type = 'track' AND save_item_id = t.track_id AND saves.user_id = parent_track.owner_id AND saves.is_delete = false
      LIMIT 10
    ) r
  )::jsonb as remix_of,


  -- followee_favorites,
  -- route_id,
  stem_of,
  track_segments, -- todo: can we just get rid of this now?
  t.updated_at,
  t.owner_id as user_id,
  t.is_delete,
  cover_art,
  is_available,
  ai_attribution_user_id,
  allowed_api_keys,
  audio_upload_id,
  preview_start_seconds,
  bpm,
  is_custom_bpm,
  musical_key,
  is_custom_musical_key,
  audio_analysis_error_count,
  comments_disabled,
  ddex_release_ids,
  artists,
  resource_contributors,
  indirect_resource_contributors,
  rights_controller,
  copyright_line,
  producer_copyright_line,
  parental_warning_type,
  -- is_streamable,
  is_stream_gated,
  stream_conditions,
  is_download_gated,
  download_conditions,
  cover_original_song_title,
  is_owned_by_user

  -- stream,
  -- download,
  -- preview




FROM tracks t
JOIN aggregate_track using (track_id)
LEFT JOIN aggregate_plays on play_item_id = t.track_id
LEFT JOIN track_routes on t.track_id = track_routes.track_id and track_routes.is_current = true
WHERE (is_unlisted = false OR t.owner_id = @my_id)
  AND t.track_id = ANY(@ids::int[])
ORDER BY t.track_id
;
