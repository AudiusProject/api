package testdata

import "time"

var UsdcTransactionsHistoryFixtures = []map[string]any{
	{
		"user_bank":        "DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D",
		"created_at":       time.Date(2021, 1, 1, 0, 0, 0, 0, time.UTC),
		"signature":        "0x12345",
		"method":           "receive",
		"transaction_type": "transfer",
		"change":           100,
		"balance":          100,
	},
	{
		"user_bank":        "DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D",
		"created_at":       time.Date(2021, 1, 2, 0, 0, 0, 0, time.UTC),
		"signature":        "0x23456",
		"method":           "send",
		"transaction_type": "purchase_content",
		"change":           10,
		"balance":          90,
	},
	{
		"user_bank":        "DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D",
		"created_at":       time.Date(2021, 1, 3, 0, 0, 0, 0, time.UTC),
		"signature":        "0x34567",
		"method":           "send",
		"transaction_type": "transfer",
		"change":           10,
		"balance":          80,
	},
	{
		"user_bank":        "DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D",
		"created_at":       time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC),
		"signature":        "0x45678",
		"method":           "send",
		"transaction_type": "prepare_withdrawal",
		"change":           10,
		"balance":          70,
	},
	{
		"user_bank":        "DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D",
		"created_at":       time.Date(2021, 1, 5, 0, 0, 0, 0, time.UTC),
		"signature":        "0x56789",
		"method":           "receive",
		"transaction_type": "recover_withdrawal",
		"change":           10,
		"balance":          80,
	},
	{
		"user_bank":        "DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D",
		"created_at":       time.Date(2021, 1, 6, 0, 0, 0, 0, time.UTC),
		"signature":        "0x67890",
		"method":           "send",
		"transaction_type": "withdrawal",
		"change":           10,
		"balance":          70,
	},
}

// user_bank,created_at,signature,method,transaction_type,change,balance
// DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D,2021-01-01T00:00:00Z,0x12345,receive,transfer,100,100
// DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D,2021-01-02T00:00:00Z,0x23456,send,purchase_content,10,90
// DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D,2021-01-03T00:00:00Z,0x34567,send,transfer,10,80
// DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D,2021-01-04T00:00:00Z,0x45678,send,prepare_withdrawal,10,70
// DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D,2021-01-05T00:00:00Z,0x56789,receive,recover_withdrawal,10,80
// DsUGy77ssRh9EXzef3AZLLT9GQBuyqHRdhkBkfqQ3x1D,2021-01-06T00:00:00Z,0x67890,send,withdrawal,10,70
