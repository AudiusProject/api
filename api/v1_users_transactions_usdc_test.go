package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetUserUsdcTransactions(t *testing.T) {
	app := testAppWithFixtures(t)
	// Default sort (reverse chronological)
	// Default excludes system transactions
	status, body := testGet(t, app, "/v1/users/7eP5n/transactions/usdc")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.method":           "send",
		"data.0.transaction_type": "withdrawal",
		"data.0.change":           float64(10),
		"data.0.balance":          float64(70),

		"data.1.method":           "send",
		"data.1.transaction_type": "transfer",
		"data.1.change":           float64(10),
		"data.1.balance":          float64(80),

		"data.2.method":           "send",
		"data.2.transaction_type": "purchase_content",
		"data.2.change":           float64(10),
		"data.2.balance":          float64(90),

		"data.3.method":           "receive",
		"data.3.transaction_type": "transfer",
		"data.3.change":           float64(100),
		"data.3.balance":          float64(100),
	})

	// include system transactions
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?include_system_transactions=true")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.method":           "send",
		"data.0.transaction_type": "withdrawal",
		"data.0.change":           float64(10),
		"data.0.balance":          float64(70),

		"data.1.method":           "receive",
		"data.1.transaction_type": "recover_withdrawal",
		"data.1.change":           float64(10),
		"data.1.balance":          float64(80),

		"data.2.method":           "send",
		"data.2.transaction_type": "prepare_withdrawal",
		"data.2.change":           float64(10),
		"data.2.balance":          float64(70),

		"data.3.method":           "send",
		"data.3.transaction_type": "transfer",
		"data.3.change":           float64(10),
		"data.3.balance":          float64(80),
	})

	// sort by date ascending
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?sort_method=date&sort_direction=asc")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.method":           "receive",
		"data.0.transaction_type": "transfer",
		"data.0.change":           float64(100),
		"data.0.balance":          float64(100),

		"data.1.method":           "send",
		"data.1.transaction_type": "purchase_content",
		"data.1.change":           float64(10),
		"data.1.balance":          float64(90),
	})

	// sort by transaction type descending
	// Secondary sort is always date descending
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?sort_method=transaction_type&sort_direction=desc")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.method":           "send",
		"data.0.transaction_type": "withdrawal",
		"data.0.change":           float64(10),
		"data.0.balance":          float64(70),

		"data.1.method":           "send",
		"data.1.transaction_type": "transfer",
		"data.1.change":           float64(10),
		"data.1.balance":          float64(80),

		"data.2.method":           "receive",
		"data.2.transaction_type": "transfer",
		"data.2.change":           float64(100),
		"data.2.balance":          float64(100),

		"data.3.method":           "send",
		"data.3.transaction_type": "purchase_content",
		"data.3.change":           float64(10),
		"data.3.balance":          float64(90),
	})

	// filter by types
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?type=transfer&type=purchase_content")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.method":           "send",
		"data.0.transaction_type": "transfer",

		"data.1.method":           "send",
		"data.1.transaction_type": "purchase_content",

		"data.2.method":           "receive",
		"data.2.transaction_type": "transfer",
	})

	// filter by method
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?method=receive")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.method":           "receive",
		"data.0.transaction_type": "transfer",
		"data.0.change":           float64(100),
		"data.0.balance":          float64(100),
	})
}

func TestGetUserUsdcTransactionsCount(t *testing.T) {
	app := testAppWithFixtures(t)

	// Default sort
	// Excludes system transactions
	status, body := testGet(t, app, "/v1/users/7eP5n/transactions/usdc/count")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data": 4,
	})

	// include system transactions
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc/count?include_system_transactions=true")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data": 6,
	})

	// filter by types
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc/count?type=transfer&type=purchase_content")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data": 3,
	})

	// filter by method
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc/count?method=receive")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data": 1,
	})
}
