package searcher

type TrackIndexer struct {
	*BaseIndexer
}

func (ti *TrackIndexer) createIndex(drop bool) error {
	// we rely on elasticsearch dynamic mapping
	// so we only need to specify properties that might be ambigious (int vs float)
	mapping := `{
		"mappings": {
			"properties": {
				"bpm":      { "type": "float" }
			}
		}
	}`
	return ti.BaseIndexer.createIndex("tracks", mapping, drop)
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
				'musical_key', musical_key,
				'bpm', bpm,
				'tags', string_to_array(tags, ','),
				'is_downloadable', is_downloadable,
				'download_conditions', download_conditions,
				'stream_conditions', stream_conditions,
				'user', json_build_object(
					'handle', users.handle,
					'name', users.name,
					'location', users.location,
					'follower_count', aggregate_user.follower_count
				)
			)
		FROM tracks
		JOIN aggregate_track USING (track_id)
		JOIN users ON owner_id = user_id
		JOIN aggregate_user USING (user_id)
		`

	return ti.bulkIndexQuery("tracks", sql)
}
