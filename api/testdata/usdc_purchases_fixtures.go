package testdata

// SolPurchasesFixtures + SolPaymentsFixtures populate the new tables backing
// v_usdc_purchases. Maintained alongside the legacy UsdcPurchasesFixtures
// during the purchases-cutover transition.
var SolPurchasesFixtures = []map[string]any{
	{
		"signature":         "a",
		"instruction_index": 0,
		"buyer_user_id":     11,
		"content_id":        303,
		"content_type":      "track",
		"amount":            135,
		"is_valid":          true,
	},
	{
		"signature":         "b",
		"instruction_index": 0,
		"buyer_user_id":     11,
		"content_id":        4,
		"content_type":      "album",
		"amount":            135,
		"is_valid":          true,
	},
}

var SolPaymentsFixtures = []map[string]any{
	{"signature": "a", "instruction_index": 0, "route_index": 0, "to_account": "0x123", "amount": 135000000, "slot": 101},
	{"signature": "b", "instruction_index": 0, "route_index": 0, "to_account": "0x123", "amount": 135000000, "slot": 101},
}
