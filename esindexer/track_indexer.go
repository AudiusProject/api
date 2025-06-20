package esindexer

type TrackIndexer struct {
	*BaseIndexer
}

func (ti *TrackIndexer) createIndex() error {
	// we rely on elasticsearch dynamic mapping
	// so we only need to specify properties that might be ambigious (int vs float)
	mapping := `{
		"mappings": {
			"properties": {
				"bpm":      { "type": "float" }
			}
		}
	}`
	return ti.BaseIndexer.createIndex("tracks", mapping)
}

func (ti *TrackIndexer) indexAll() error {
	sql := `
		SELECT
			track_id,
			json_build_object(
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
		AND tracks.blocknumber > $1
		ORDER BY tracks.blocknumber ASC

		-- LIMIT 1000
		`

	return ti.bulkIndexQuery("tracks", sql)
}
