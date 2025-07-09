package esindexer

var tracksConfig = collectionConfig{
	indexName: "tracks",
	idColumn:  "track_id",
	mapping: `
	{
		"mappings": {
			"properties": {
				"bpm":      { "type": "float" }
			}
		}
	}`,
	sql: `
	SELECT
		track_id,
		json_build_object(
			'suggest', CONCAT_WS(' ', title, users.name, users.handle),
			'title', title,
			'genre', genre,
			'mood', mood,
			'duration', duration,
			'save_count', aggregate_track.save_count,
			'repost_count', aggregate_track.repost_count,
			'comment_count', aggregate_track.comment_count,
			'release_date', coalesce(release_date, tracks.created_at),
			'updated_at', tracks.updated_at,
			'blocknumber', tracks.blocknumber,
			'musical_key', musical_key,
			'bpm', bpm,
			'tags', string_to_array(tags, ','),
			'is_downloadable', is_downloadable,
			'has_stems', (select true from stems where parent_track_id = tracks.track_id limit 1),
			'download_conditions', download_conditions,
			'stream_conditions', stream_conditions,
			'user', json_build_object(
				'handle', users.handle,
				'name', users.name,
				'location', users.location,
				'follower_count', aggregate_user.follower_count,
				'is_verified', is_verified
			)
		)
	FROM tracks
	JOIN aggregate_track USING (track_id)
	JOIN users ON owner_id = user_id
	JOIN aggregate_user USING (user_id)
	WHERE tracks.is_unlisted = false
	AND tracks.is_delete = false
	AND tracks.is_available = true
	AND users.is_available = true
	AND users.is_deactivated = false
	AND stem_of is null
	`,
}
