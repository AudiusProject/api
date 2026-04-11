begin;

alter table aggregate_user add column if not exists total_playlist_count int default 0;
alter table aggregate_user add column if not exists total_album_count int default 0;

UPDATE aggregate_user au
SET total_playlist_count = playlist_stats.total_count
FROM (
  SELECT
    p.playlist_owner_id AS user_id,
    COUNT(*) AS total_count
  FROM playlists p
  WHERE
    p.is_current IS TRUE
    AND p.is_delete IS FALSE
    AND p.is_album IS FALSE
  GROUP BY p.playlist_owner_id
) AS playlist_stats
WHERE au.user_id = playlist_stats.user_id;

UPDATE aggregate_user au
SET total_album_count = album_stats.total_count
FROM (
  SELECT
    p.playlist_owner_id AS user_id,
    COUNT(*) AS total_count
  FROM playlists p
  WHERE
    p.is_current IS TRUE
    AND p.is_delete IS FALSE
    AND p.is_album IS TRUE
  GROUP BY p.playlist_owner_id
) AS album_stats
WHERE au.user_id = album_stats.user_id;

commit;
