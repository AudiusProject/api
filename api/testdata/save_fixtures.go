package testdata

var SaveFixtures = []map[string]any{
	{
		"user_id":        1,
		"save_item_id":   1,
		"save_item_type": "playlist",
	},
	{
		"user_id":        1,
		"save_item_id":   2,
		"save_item_type": "album",
	},
	{
		"user_id":        1,
		"save_item_id":   100,
		"save_item_type": "track",
	},
}

// user_id,save_item_id,save_item_type
// 1,1,'playlist'
// 1,2,'album'
// 1,100,'track'
