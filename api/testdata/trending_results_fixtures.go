package testdata

var TrendingResultsFixtures = []map[string]any{
	// Week 2022-01-21 - tracks 300, 202, 200 (rank 1, 2, 3) - matches trending order
	{"user_id": 3, "id": "300", "rank": 1, "type": "TrendingType.TRACKS", "version": "TrendingVersion.ML51L", "week": "2022-01-21"},
	{"user_id": 2, "id": "202", "rank": 2, "type": "TrendingType.TRACKS", "version": "TrendingVersion.ML51L", "week": "2022-01-21"},
	{"user_id": 2, "id": "200", "rank": 3, "type": "TrendingType.TRACKS", "version": "TrendingVersion.ML51L", "week": "2022-01-21"},
}
