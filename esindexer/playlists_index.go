package esindexer

var playlistsConfig = collectionConfig{
	indexName: "playlists",
	idColumn:  "playlist_id",
	sql: `
	SELECT
		playlist_id,
		json_build_object(
			'suggest', CONCAT_WS(' ', playlist_name, users.name, users.handle),
			'title', playlist_name,
			'description', description,
			'track_count', track_count,
			'save_count', aggregate_playlist.save_count,
			'repost_count', aggregate_playlist.repost_count,
			'created_at', playlists.created_at,
			'updated_at', playlists.updated_at,
			'blocknumber', playlists.blocknumber,
			'is_private', is_private,
			'is_album', playlists.is_album,
			'user', json_build_object(
				'handle', users.handle,
				'name', users.name,
				'location', users.location,
				'follower_count', aggregate_user.follower_count,
				'is_verified', is_verified
			),
			'tracks', (
				SELECT json_agg(
					json_build_object(
						'title', title,
						'genre', genre,
						'mood', mood,
						'tags', string_to_array(tags, ','),

						'user', json_build_object(
							'handle', users.handle,
							'name', users.name,
							'location', users.location,
							'follower_count', aggregate_user.follower_count,
							'is_verified', is_verified
						)
					)
				)
				FROM playlist_tracks
				JOIN tracks USING (track_id)
				JOIN users ON owner_id = user_id
				WHERE playlist_id = playlists.playlist_id
			)
		)
	FROM playlists
	JOIN aggregate_playlist USING (playlist_id)
	JOIN users ON playlist_owner_id = user_id
	JOIN aggregate_user USING (user_id)
	WHERE is_private = false
	AND users.is_available = true
	AND users.is_deactivated = false
	`,
}
