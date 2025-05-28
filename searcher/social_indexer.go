package searcher

// user id => list of following, repost (tracks), save (tracks), repost (playlists), repost (playlist)
type SocialIndexer struct {
	*BaseIndexer
}

func (ui *SocialIndexer) createIndex(drop bool) error {
	mapping := `{
		"mappings": {
			"properties": {
				"saved_track_ids": { "type": "keyword" },
				"reposted_track_ids": { "type": "keyword" },
				"saved_playlist_ids": { "type": "keyword" },
				"reposted_playlist_ids": { "type": "keyword" },
				"following_user_ids": { "type": "keyword" }
			}
		}
	}`
	return ui.BaseIndexer.createIndex("socials", mapping, drop)
}

func (ui *SocialIndexer) indexAll() error {
	sql := `
	select
	    user_id,
	    json_build_object (
	        'saved_track_ids',
	        (
	            select
	                array_agg (save_item_id)
	            from
	                saves
	            where
	                save_type = 'track'
	                and is_delete = false
	                and user_id = users.user_id
	        ),
	        'reposted_track_ids',
	        (
	            select
	                array_agg (repost_item_id)
	            from
	                reposts
	            where
	                repost_type = 'track'
	                and is_delete = false
	                and user_id = users.user_id
	        ),
	        'saved_playlist_ids',
	        (
	            select
	                array_agg (save_item_id)
	            from
	                saves
	            where
	                save_type != 'track'
	                and is_delete = false
	                and user_id = users.user_id
	        ),
	        'reposted_playlist_ids',
	        (
	            select
	                array_agg (repost_item_id)
	            from
	                reposts
	            where
	                repost_type != 'track'
	                and is_delete = false
	                and user_id = users.user_id
	        ),
	        'following_user_ids',
	        (
	            select
	                array_agg (followee_user_id)
	            from
	                follows
	            where
	                is_delete = false
	                and follower_user_id = users.user_id
	        )
	    ) as doc
	from
	    users
	where
	    user_id < 1000
	`

	return ui.bulkIndexQuery("socials", sql)
}
