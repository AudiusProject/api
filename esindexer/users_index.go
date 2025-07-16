package esindexer

var userConfig = collectionConfig{
	indexName: "users",
	idColumn:  "user_id",
	sql: `
	SELECT
		user_id,
		json_build_object(
			'suggest', CONCAT_WS(' ', name, handle),
			'name', name,
			'handle', handle,
			'bio', bio,
			'location', location,
			'created_at', created_at,
			'updated_at', updated_at,
			'blocknumber', users.blocknumber,
			'is_verified', is_verified,

			'track_count', track_count,
			'playlist_count', playlist_count,
			'follower_count', follower_count,
			'repost_count', repost_count,
			'track_save_count', track_save_count,
			'supporter_count', supporter_count,
			'supporting_count', supporting_count,
			'dominant_genre', dominant_genre,
			'dominant_genre_count', dominant_genre_count,

			'tracks', (
				SELECT json_agg(
					json_build_object(
						'title', title,
						'genre', genre,
						'mood', mood,
						'tags', string_to_array(tags, ',')

						-- todo: more track fields
					)
				)
				FROM tracks
				WHERE owner_id = users.user_id
			)
		)
	FROM users
	JOIN aggregate_user USING (user_id)
	WHERE is_deactivated = false
	AND is_available = true
	`,
}
