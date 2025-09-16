package testdata

var ArtistCoinsFixtures = []map[string]any{
	{
		"ticker":     "$TESTCOIN",
		"decimals":   8,
		"user_id":    1, // Default user (rayjacobson)
		"mint":       "test_mint_address_123",
		"logo_uri":   "https://example.com/test-logo.png",
		"created_at": "2024-01-01 00:00:00",
	},
	{
		"ticker":     "$AUDIO",
		"decimals":   8,
		"user_id":    2, // stereosteve
		"mint":       "9LzCMqDgTKYz9Drzqnpgee3SGa89up3a247ypMj2xrqM",
		"logo_uri":   "https://example.com/audio-logo.png",
		"created_at": "2024-01-01 00:00:00",
	},
}
