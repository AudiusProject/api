package testdata

var Playlists = []map[string]any{
	{
		"playlist_id":       1,
		"playlist_name":     "First",
		"playlist_owner_id": 1,
		"is_album":          false,
		"playlist_contents": `{"track_ids": [{"time": 1722451644, "track": 200, "metadata_time": 1722451644},{"time": 1722451644, "track": -1, "metadata_time": 1722451644},{"time": 1722451644, "track": 300, "metadata_time": 1722451644}]}`,
		"stream_conditions": nil,
	},
	{
		"playlist_id":       2,
		"playlist_name":     "Follow Gated Stream",
		"playlist_owner_id": 3,
		"is_album":          true,
		"playlist_contents": `{}`,
		"stream_conditions": `{"follow_user_id": 3}`,
	},
	{
		"playlist_id":       3,
		"playlist_name":     "SecondAlbum",
		"playlist_owner_id": 1,
		"is_album":          true,
		"playlist_contents": `{"track_ids": [{"time": 1722451644, "track": 200, "metadata_time": 1722451644},{"time": 1722451644, "track": -1, "metadata_time": 1722451644},{"time": 1722451644, "track": 300, "metadata_time": 1722451644}]}`,
		"stream_conditions": nil,
	},
	{
		"playlist_id":       4,
		"playlist_name":     "Purchase Gated Stream",
		"playlist_owner_id": 3,
		"is_album":          true,
		"playlist_contents": `{}`,
		"stream_conditions": `{"usdc_purchase": {"price": 135, "splits": [{"user_id": 3, "percentage": 100.0}]}}`,
	},
	{
		"playlist_id":       500,
		"playlist_name":     "playlist by permalink",
		"playlist_owner_id": 7,
		"is_album":          false,
		"playlist_contents": nil,
		"stream_conditions": nil,
	},
	{
		"playlist_id":       501,
		"playlist_name":     "album by permalink",
		"playlist_owner_id": 8,
		"is_album":          true,
		"playlist_contents": nil,
		"stream_conditions": nil,
	},
}
