package api

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// USDC transactions are sourced from v_token_transactions_history. Every
// legacy transaction_type now flows through end-to-end:
//
//   purchase_content       — sol_purchases row
//   transfer               — bare sol_claimable_account_transfers (no memo)
//   withdrawal             — claimable transfer with "Withdrawal" memo marker
//   prepare_withdrawal     — claimable transfer with "Prepare Withdrawal" memo
//                            (or Jupiter program touched in the tx)
//   recover_withdrawal     — payment_router route with "Recover Withdrawal" memo
//   internal_transfer      — claimable transfer with "Internal Transfer" memo
//   purchase_stripe etc.   — supported on the AUDIO mint, validated by the
//                            v_token_transactions_history view tests; USDC
//                            top-ups arrive via payment_router (purchase_content)
//                            so these query-types validate without rows.
//
// The fixtures in sol_usdc_transactions_fixtures.go seed one of each except
// the purchase_* USDC variants (which aren't a thing on chain — USDC top-ups
// go through the payment-router purchase_content path).
func TestGetUserUsdcTransactions(t *testing.T) {
	app := testAppWithFixtures(t)

	// Default sort (transaction_date desc). System-transaction filter excludes
	// prepare_withdrawal + recover_withdrawal.
	status, body := testGet(t, app, "/v1/users/7eP5n/transactions/usdc")
	assert.Equal(t, 200, status)

	jsonAssert(t, body, map[string]any{
		"data.#": 5, // 7 rows total minus prepare_withdrawal + recover_withdrawal

		// 0x90123 → internal_transfer (2024-06-08)
		"data.0.method":           "send",
		"data.0.transaction_type": "internal_transfer",
		"data.0.change":           float64(300000),

		// 0x67890 → withdrawal (2024-06-05)
		"data.1.method":           "send",
		"data.1.transaction_type": "withdrawal",
		"data.1.change":           float64(1000000),

		// 0x34567 → transfer (2024-06-04)
		"data.2.method":           "send",
		"data.2.transaction_type": "transfer",
		"data.2.change":           float64(500000),

		// 0x23456 → purchase_content (2021-01-02)
		"data.3.method":           "send",
		"data.3.transaction_type": "purchase_content",
		"data.3.change":           float64(10),

		// 0x12345 → transfer (2021-01-01)
		"data.4.method":           "receive",
		"data.4.transaction_type": "transfer",
		"data.4.change":           float64(100),
	})

	// Include system transactions — surfaces prepare_withdrawal + recover_withdrawal.
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?include_system_transactions=true")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data.#": 7})

	// Filter by withdrawal — returns the one tagged withdrawal row.
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?type=withdrawal")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#":                  1,
		"data.0.transaction_type": "withdrawal",
		"data.0.change":           float64(1000000),
	})

	// Filter by prepare_withdrawal — only surfaces when include_system_transactions=true.
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?type=prepare_withdrawal&include_system_transactions=true")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#":                  1,
		"data.0.transaction_type": "prepare_withdrawal",
	})

	// Filter by recover_withdrawal — same.
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?type=recover_withdrawal&include_system_transactions=true")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#":                  1,
		"data.0.transaction_type": "recover_withdrawal",
		"data.0.method":           "receive",
	})

	// Filter by internal_transfer — present without include_system_transactions
	// because internal_transfer wasn't in the legacy system-filter set.
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?type=internal_transfer")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#":                  1,
		"data.0.transaction_type": "internal_transfer",
	})

	// Filter by method.
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?method=receive")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.#":                  1,
		"data.0.method":           "receive",
		"data.0.transaction_type": "transfer",
		"data.0.change":           float64(100),
	})

	// Filter by method with system included — adds recover_withdrawal (also a
	// `receive` from the user's perspective).
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?method=receive&include_system_transactions=true")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data.#": 2})

	// Sort by date ascending.
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc?sort_method=date&sort_direction=asc")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{
		"data.0.transaction_type": "transfer",
		"data.0.method":           "receive",
	})
}

func TestGetUserUsdcTransactionsCount(t *testing.T) {
	app := testAppWithFixtures(t)

	// Default excludes the two system types.
	status, body := testGet(t, app, "/v1/users/7eP5n/transactions/usdc/count")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data": 5})

	// Including system types: all 7.
	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc/count?include_system_transactions=true")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data": 7})

	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc/count?type=transfer&type=purchase_content")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data": 3})

	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc/count?type=withdrawal")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data": 1})

	status, body = testGet(t, app, "/v1/users/7eP5n/transactions/usdc/count?method=receive")
	assert.Equal(t, 200, status)
	jsonAssert(t, body, map[string]any{"data": 1})
}
