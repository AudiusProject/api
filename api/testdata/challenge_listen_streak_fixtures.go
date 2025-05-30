package testdata

import "time"

func timePointer(s string) *time.Time {
	t, _ := time.Parse("2006-01-02 15:04:05", s)
	return &t
}

var ChallengeListenStreak = []map[string]any{
	{
		"user_id":          400,
		"listen_streak":    2,
		"last_listen_date": nil,
	},
	{
		"user_id":          401,
		"listen_streak":    3,
		"last_listen_date": nil,
	},
	{
		"user_id":          402,
		"listen_streak":    5,
		"last_listen_date": nil,
	},
	{
		"user_id":          403,
		"listen_streak":    6,
		"last_listen_date": timePointer("2006-01-02 15:04:07"),
	},
}
