package searcher

type PlaylistIndexer struct {
	*BaseIndexer
}

func (pi *PlaylistIndexer) createIndex(drop bool) error {
	// Specify mappings for playlist index
	// Only specifying properties that might be ambiguous
	mapping := `{
		"mappings": {
			"properties": {
			}
		}
	}`
	return pi.BaseIndexer.createIndex("playlists", mapping, drop)
}

func (pi *PlaylistIndexer) indexAll() error {
	// todo: track stubs
	sql := `
		SELECT
			playlist_id,
			json_build_object(
				'title', playlist_name,
				'description', description,
				'track_count', track_count,
				'save_count', aggregate_playlist.save_count,
				'repost_count', aggregate_playlist.repost_count,
				'last_updated', playlists.updated_at,
				'created_at', playlists.created_at,
				'is_private', is_private,
				'is_album', playlists.is_album,
				'user', json_build_object(
					'handle', users.handle,
					'name', users.name,
					'location', users.location,
					'follower_count', aggregate_user.follower_count
				)
			)
		FROM playlists
		JOIN aggregate_playlist USING (playlist_id)
		JOIN users ON playlist_owner_id = user_id
		JOIN aggregate_user USING (user_id)
		WHERE is_private = false
		AND users.is_available = true
		AND users.is_deactivated = false
		`

	return pi.bulkIndexQuery("playlists", sql)
}
