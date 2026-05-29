package testdata

var SaveFixtures = []map[string]any{
	{
		"user_id":      1,
		"save_item_id": 1,
		"save_type":    "playlist",
	},
	{
		"user_id":      1,
		"save_item_id": 2,
		"save_type":    "playlist",
	},
	{
		"user_id":      1,
		"save_item_id": 100,
		"save_type":    "track",
	},
	// Saves of a deleted track and a track owned by a deactivated user.
	// The /users/:id/favorites endpoint must filter both out.
	{
		"user_id":      1,
		"save_item_id": 950,
		"save_type":    "track",
	},
	{
		"user_id":      1,
		"save_item_id": 951,
		"save_type":    "track",
	},
}

// user_id,save_item_id,save_item_type
// 1,1,'playlist'
// 1,2,'playlist'
// 1,100,'track'
