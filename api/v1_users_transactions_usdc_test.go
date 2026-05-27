package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// USDC transactions are now sourced from v_token_transactions_history. Only
// the transaction types derivable from sol_* tables surface here:
// `purchase_content` (sol_purchases) and `transfer` (sol_claimable_account_transfers).
// The legacy `prepare_withdrawal` / `recover_withdrawal` system-level events
// have no underlying balance change and disappear entirely; `withdrawal` rows
// reappear as bare `transfer` because the Go indexer doesn't (yet) emit
// withdrawal-specific metadata. `purchase_stripe` / `internal_transfer` are
// similarly absent until vendor-memo capture lands.
func TestGetUserUsdcTransactions(t *testing.T) {
	app := testAppWithFixtures(t)

	// Default sort (transaction_date desc). System-transaction filter is a no-op
	// against the view since system types are not derivable.
	status, body := testGet(t, app, "/v1/users/7eP5n/transactions/usdc")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#": 4,

		"data.0.method":           "send",
		"data.0.transaction_type": "transfer",
		"data.0.change":           float64(1000000),
		"data.0.balance":          float64(70),

		"data.1.method":           "send",
		"data.1.transaction_type": "transfer",
		"data.1.change":           float64(500000),
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

	// sort by date ascending
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?sort_method=date&sort_direction=asc")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.method":           "receive",
		"data.0.transaction_type": "transfer",
		"data.0.balance":          float64(100),

		"data.1.method":           "send",
		"data.1.transaction_type": "purchase_content",
		"data.1.balance":          float64(90),
	})

	// sort by transaction type descending — `transfer` sorts before
	// `purchase_content`. Within a type, secondary sort is date desc.
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?sort_method=transaction_type&sort_direction=desc")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.0.transaction_type": "transfer",
		"data.0.balance":          float64(70),

		"data.1.transaction_type": "transfer",
		"data.1.balance":          float64(80),

		"data.2.transaction_type": "transfer",
		"data.2.balance":          float64(100),

		"data.3.transaction_type": "purchase_content",
		"data.3.balance":          float64(90),
	})

	// filter by types
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?type=transfer&type=purchase_content")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data.#": 4})

	// filter by method
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?method=receive")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#":                  1,
		"data.0.method":           "receive",
		"data.0.transaction_type": "transfer",
		"data.0.balance":          float64(100),
	})

	// Filtering by a withdrawal type still validates as a known transaction
	// type, just returns no rows (no derivable mapping in v_token_transactions_history).
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?type=withdrawal")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data.#": 0})
}

func TestGetUserUsdcTransactionsCount(t *testing.T) {
	app := testAppWithFixtures(t)

	status, body := testGet(t, app, "/v1/users/7eP5n/transactions/usdc/count")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data": 4})

	// include_system_transactions is a no-op for the view-backed query — system
	// types simply don't exist in the data.
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc/count?include_system_transactions=true")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data": 4})

	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc/count?type=transfer&type=purchase_content")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data": 4})

	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc/count?method=receive")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data": 1})
}
