package testdata

var AggregateUser = []map[string]any{
	{
		"user_id":         1,
		"follower_count":  100,
		"following_count": 25,
		"dominant_genre":  "Electronic",
		// Seed the pre-save baseline (0): the on_save trigger is delta-based
		// (since #898) and increments this as the 3 track saves in SaveFixtures
		// are inserted, yielding a final track_save_count of 3.
		"track_save_count": 0,
	},
	{
		"user_id":          2,
		"follower_count":   50,
		"following_count":  15,
		"dominant_genre":   "Electronic",
		"track_save_count": 0,
	},
	{
		"user_id":          3,
		"follower_count":   20,
		"following_count":  10,
		"dominant_genre":   "Electronic",
		"track_save_count": 0,
	},
	{
		"user_id":          301,
		"follower_count":   2000,
		"following_count":  500,
		"dominant_genre":   "Electronic",
		"track_save_count": 0,
	},
	{
		"user_id":          302,
		"follower_count":   500,
		"following_count":  2000,
		"dominant_genre":   "Electronic",
		"track_save_count": 0,
	},
}
