package testdata

import "time"

var UserListeningHistoryFixtures = []map[string]any{
	{
		"user_id": 410,
		"listening_history": []map[string]any{
			{
				"track_id":   100,
				"play_count": 5,
				"timestamp":  time.Date(2024, 6, 2, 14, 0, 0, 0, time.UTC),
			},
			{
				"track_id":   101,
				"play_count": 2,
				"timestamp":  time.Date(2024, 6, 2, 13, 0, 0, 0, time.UTC),
			},
			{
				"track_id":   507,
				"play_count": 20,
				"timestamp":  time.Date(2024, 6, 2, 12, 0, 0, 0, time.UTC),
			},
		},
	},
}
