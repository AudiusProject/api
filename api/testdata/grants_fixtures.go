package testdata

var Grants = []map[string]any{
	{
		"user_id":         1,
		"grantee_address": "0x5f1a372b28956c8363f8bc3a231a6e9e1186ead8",
		"is_approved":     true,
		"is_revoked":      false,
	},
	{
		"user_id":         1,
		"grantee_address": "0xc451c1f8943b575158310552b41230c61844a1c1",
		"is_approved":     false,
		"is_revoked":      true,
	},
	{
		"user_id":         1,
		"grantee_address": "0x1234567890abcdef",
		"is_approved":     true,
		"is_revoked":      true,
	},
	{
		"user_id":         1,
		"grantee_address": "0x681c616ae836ceca1effe00bd07f2fdbf9a082bc",
		"is_approved":     false,
		"is_revoked":      false,
	},
	{
		"user_id":         2,
		"grantee_address": "0x681c616ae836ceca1effe00bd07f2fdbf9a082bc",
		"is_approved":     true,
		"is_revoked":      false,
	},
	{
		"user_id":         3,
		"grantee_address": "0x681c616ae836ceca1effe00bd07f2fdbf9a082bc",
		"is_approved":     true,
		"is_revoked":      true,
	},
	{
		"user_id":         4,
		"grantee_address": "0x681c616ae836ceca1effe00bd07f2fdbf9a082bc",
		"is_approved":     false,
		"is_revoked":      true,
	},
}
