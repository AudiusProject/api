package testdata

var LatestTrackIds = []int{802, 801, 800}

var LatestTrackFixtures = []map[string]any{
	{"track_id": 800, "genre": "LatestTestGenreA", "owner_id": 1, "title": "Latest Track Old", "is_unlisted": "f", "created_at": "2025-01-01 00:00:00"},
	{"track_id": 801, "genre": "LatestTestGenreA", "owner_id": 1, "title": "Latest Track Mid", "is_unlisted": "f", "created_at": "2025-02-01 00:00:00"},
	{"track_id": 802, "genre": "LatestTestGenreC", "owner_id": 1, "title": "Latest Track New", "is_unlisted": "f", "created_at": "2025-03-01 00:00:00"},
	{"track_id": 803, "genre": "LatestTestGenreB", "owner_id": 1, "title": "Latest Track Visible", "is_unlisted": "f", "created_at": "2025-02-15 00:00:00"},
	{"track_id": 804, "genre": "LatestTestGenreB", "owner_id": 1, "title": "Latest Track Hidden", "is_unlisted": "t", "created_at": "2025-02-20 00:00:00"},
}
