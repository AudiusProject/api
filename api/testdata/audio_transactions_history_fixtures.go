package testdata

import "time"

var AudioTransactionsHistory = []map[string]any{
	{
		"user_bank":        "DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D",
		"created_at":       time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		"signature":        "0x12345",
		"method":           "receive",
		"transaction_type": "tip",
		"change":           100,
		"balance":          100,
	},
	{
		"user_bank":        "DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D",
		"created_at":       time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
		"signature":        "0x23456",
		"method":           "send",
		"transaction_type": "tip",
		"change":           10,
		"balance":          90,
	},
	{
		"user_bank":        "DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D",
		"created_at":       time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC),
		"signature":        "0x34567",
		"method":           "send",
		"transaction_type": "tip",
		"change":           10,
		"balance":          80,
	},
	{
		"user_bank":        "DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D",
		"created_at":       time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC),
		"signature":        "0x45678",
		"method":           "send",
		"transaction_type": "transfer",
		"change":           50,
		"balance":          30,
	},
	{
		"user_bank":        "DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D",
		"created_at":       time.Date(2021, 1, 5, 0, 0, 0, 0, time.UTC),
		"signature":        "0x56789",
		"method":           "send",
		"transaction_type": "transfer",
		"change":           10,
		"balance":          20,
	},
}
