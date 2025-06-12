package searcher

/*

stage:
http://35.239.100.60:21302

prod:
http://35.238.44.255:21302

*/

type UserIndexer struct {
	*BaseIndexer
}

func (ui *UserIndexer) createIndex(drop bool) error {
	mapping := ``
	return ui.BaseIndexer.createIndex("users", mapping, drop)
}

func (ui *UserIndexer) indexAll() error {
	sql := `
		SELECT
			user_id,
			json_build_object(
				'name', name,
				'handle', handle,
				'bio', bio,
				'location', location,
				'created_at', created_at,
				'is_verified', is_verified,

				'track_count', track_count,
				'playlist_count', playlist_count,
				'follower_count', follower_count,
				'repost_count', repost_count,
				'track_save_count', track_save_count,
				'supporter_count', supporter_count,
				'supporting_count', supporting_count,
				'dominant_genre', dominant_genre,
				'dominant_genre_count', dominant_genre_count
			)
		FROM users
		JOIN aggregate_user USING (user_id)
		`

	return ui.bulkIndexQuery("users", sql)
}
