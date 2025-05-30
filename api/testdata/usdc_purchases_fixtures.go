package testdata

var UsdcPurchasesFixtures = []map[string]any{
	{
		"signature":      "a",
		"buyer_user_id":  11,
		"seller_user_id": 3,
		"content_id":     303,
		"content_type":   "track",
		"amount":         135,
		"splits": []map[string]any{
			{
				"amount":     135000000,
				"user_id":    3,
				"eth_wallet": "0x123",
				"percentage": 100,
			},
		},
	},
	{
		"signature":      "b",
		"buyer_user_id":  11,
		"seller_user_id": 3,
		"content_id":     4,
		"content_type":   "album",
		"amount":         135,
		"splits": []map[string]any{
			{
				"amount":     135000000,
				"user_id":    3,
				"eth_wallet": "0x123",
				"percentage": 100,
			},
		},
	},
}

// signature,buyer_user_id,seller_user_id,content_id,content_type,amount,splits
// a,11,3,303,track,135,"[{""amount"": 135000000, ""user_id"": 3, ""eth_wallet"": ""0x123"", ""percentage"": 100}]"
// b,11,3,4,album,135,"[{""amount"": 135000000, ""user_id"": 3, ""eth_wallet"": ""0x123"", ""percentage"": 100}]"
