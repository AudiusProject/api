package testdata

import (
	"time"
)

var UserChallengesFixtures = []map[string]any{
	{
		"challenge_id":          "e",
		"user_id":               400,
		"specifier":             "abc",
		"is_complete":           false,
		"current_step_count":    2,
		"completed_blocknumber": nil,
		"amount":                2,
	},
	{
		"challenge_id":          "e",
		"user_id":               401,
		"specifier":             "def",
		"is_complete":           true,
		"current_step_count":    3,
		"completed_blocknumber": 1,
		"amount":                3,
		"completed_at":          time.Date(2006, 1, 2, 15, 4, 4, 0, time.UTC),
	},
	{
		"challenge_id":          "e",
		"user_id":               402,
		"specifier":             "cbc",
		"is_complete":           true,
		"current_step_count":    3,
		"completed_blocknumber": 2,
		"amount":                3,
		"completed_at":          time.Date(2006, 1, 2, 15, 4, 5, 0, time.UTC),
	},
	{
		"challenge_id":          "e",
		"user_id":               402,
		"specifier":             "cba",
		"is_complete":           true,
		"current_step_count":    1,
		"completed_blocknumber": 3,
		"amount":                1,
		"completed_at":          time.Date(2006, 1, 2, 15, 4, 6, 0, time.UTC),
	},
	{
		"challenge_id":          "e",
		"user_id":               402,
		"specifier":             "cbb",
		"is_complete":           true,
		"current_step_count":    1,
		"completed_blocknumber": 4,
		"amount":                1,
		"completed_at":          time.Date(2006, 1, 2, 15, 4, 7, 0, time.UTC),
	},
	{
		"challenge_id":          "e",
		"user_id":               403,
		"specifier":             "ddd",
		"is_complete":           true,
		"current_step_count":    3,
		"completed_blocknumber": 5,
		"amount":                3,
		"completed_at":          time.Date(2006, 1, 2, 15, 4, 7, 0, time.UTC),
	},
	{
		"challenge_id":          "e",
		"user_id":               403,
		"specifier":             "dde",
		"is_complete":           true,
		"current_step_count":    1,
		"completed_blocknumber": 6,
		"amount":                1,
		"completed_at":          time.Date(2006, 1, 2, 15, 4, 8, 0, time.UTC),
	},
	{
		"challenge_id":          "f",
		"user_id":               402,
		"specifier":             "fff",
		"is_complete":           true,
		"current_step_count":    0,
		"completed_blocknumber": 7,
		"amount":                1,
		"completed_at":          time.Date(2006, 1, 2, 15, 4, 9, 0, time.UTC),
	},
}

// challenge_id,user_id,specifier,is_complete,current_step_count,completed_blocknumber,amount,completed_at
// e,400,abc,f,2,,2,
// e,401,def,t,3,1,3,2006-01-02 15:04:04
// e,402,cbc,t,3,2,3,2006-01-02 15:04:05
// e,402,cba,t,1,3,1,2006-01-02 15:04:06
// e,402,cbb,t,1,4,1,2006-01-02 15:04:07
// e,403,ddd,t,3,5,3,2006-01-02 15:04:07
// e,403,dde,t,1,6,1,2006-01-02 15:04:08
// f,402,fff,t,0,7,1,2006-01-02 15:04:09
