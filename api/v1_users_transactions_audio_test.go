package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUserAudioTransactions(t *testing.T) {
	app := testAppWithFixtures(t)
	// Default sort (reverse chronological)
	status, body := testGet(t, app, "/v1/users/7eP5n/transactions/audio")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.method":           "send",
		"data.0.transaction_type": "transfer",
		"data.0.change":           float64(10),
		"data.0.balance":          float64(20),

		"data.1.method":           "send",
		"data.1.transaction_type": "transfer",
		"data.1.change":           float64(50),
		"data.1.balance":          float64(30),

		"data.2.method":           "send",
		"data.2.transaction_type": "tip",
		"data.2.change":           float64(10),
		"data.2.balance":          float64(80),

		"data.3.method":           "send",
		"data.3.transaction_type": "tip",
		"data.3.change":           float64(10),
		"data.3.balance":          float64(90),

		"data.4.method":           "receive",
		"data.4.transaction_type": "tip",
		"data.4.change":           float64(100),
		"data.4.balance":          float64(100),
	})

	// sort by date ascending
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/audio?sort_method=date&sort_direction=asc")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.method":           "receive",
		"data.0.transaction_type": "tip",
		"data.0.change":           float64(100),
		"data.0.balance":          float64(100),

		"data.1.method":           "send",
		"data.1.transaction_type": "tip",
		"data.1.change":           float64(10),
		"data.1.balance":          float64(90),
	})

	// sort by transaction type descending
	// Secondary sort is always date descending
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/audio?sort_method=transaction_type&sort_direction=desc")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.method":           "send",
		"data.0.transaction_type": "transfer",
		"data.0.change":           float64(10),
		"data.0.balance":          float64(20),

		"data.1.method":           "send",
		"data.1.transaction_type": "transfer",
		"data.1.change":           float64(50),
		"data.1.balance":          float64(30),

		"data.2.method":           "send",
		"data.2.transaction_type": "tip",
		"data.2.change":           float64(10),
		"data.2.balance":          float64(80),

		"data.3.method":           "send",
		"data.3.transaction_type": "tip",
		"data.3.change":           float64(10),
		"data.3.balance":          float64(90),

		"data.4.method":           "receive",
		"data.4.transaction_type": "tip",
		"data.4.change":           float64(100),
		"data.4.balance":          float64(100),
	})
}

func TestGetUserAudioTransactionsCount(t *testing.T) {
	app := testAppWithFixtures(t)

	// Default sort
	status, body := testGet(t, app, "/v1/users/7eP5n/transactions/audio/count")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data": 5,
	})
}
