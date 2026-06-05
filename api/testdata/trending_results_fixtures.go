package testdata

var TrendingResultsFixtures = []map[string]any{
	// Week 2022-01-21 - tracks 300, 202, 200 (rank 1, 2, 3) - matches trending order
	{"user_id": 3, "id": "300", "rank": 1, "type": "TRACKS", "version": "TrendingVersion.ML51L", "week": "2022-01-21"},
	{"user_id": 2, "id": "202", "rank": 2, "type": "TRACKS", "version": "TrendingVersion.ML51L", "week": "2022-01-21"},
	{"user_id": 2, "id": "200", "rank": 3, "type": "TRACKS", "version": "TrendingVersion.ML51L", "week": "2022-01-21"},
	// Week 2022-01-21 - underground tracks 519, 520, 521 (rank 1, 2, 3)
	{"user_id": 8, "id": "519", "rank": 1, "type": "UNDERGROUND_TRACKS", "version": "TrendingVersion.ML51L", "week": "2022-01-21"},
	{"user_id": 11, "id": "520", "rank": 2, "type": "UNDERGROUND_TRACKS", "version": "TrendingVersion.ML51L", "week": "2022-01-21"},
	{"user_id": 1, "id": "521", "rank": 3, "type": "UNDERGROUND_TRACKS", "version": "TrendingVersion.ML51L", "week": "2022-01-21"},
}
