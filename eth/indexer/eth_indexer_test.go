package indexer

import (
	"context"
	"math/big"
	"testing"

	"api.audius.co/database"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// migration0203SQL inlines the eth_wallet_balances table definition from
// ddl/migrations/0203_eth_wallet_balances.sql. Inlined rather than read
// from disk so the test stays self-contained — sql/01_schema.sql doesn't
// include this table yet (the schema dump regenerator was the broken
// `make test-schema` path), and we want the test runnable against the
// default test_jobs template.
const migration0203SQL = `
CREATE TABLE IF NOT EXISTS eth_wallet_balances (
    wallet TEXT PRIMARY KEY,
    balance NUMERIC NOT NULL DEFAULT 0,
    blocknumber BIGINT,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
`

// TestUpsertBalanceUpdates_BlockSemantics pins the three orderings that
// the block-preserve SQL has to handle correctly. The semantics matter
// because the stale-refresh sweep upserts with block=0 (translated to
// NULL by NULLIF), and we don't want a stale-refresh to overwrite a real
// blocknumber persisted by an earlier Transfer event upsert.
func TestUpsertBalanceUpdates_BlockSemantics(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	ctx := context.Background()
	_, err := pool.Exec(ctx, migration0203SQL)
	require.NoError(t, err)

	e := &EthIndexer{pool: pool}
	addr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	walletKey := lowerHex(addr)

	read := func(t *testing.T) (string, *int64) {
		t.Helper()
		var balance string
		var block *int64
		err := pool.QueryRow(ctx,
			`SELECT balance::text, blocknumber FROM eth_wallet_balances WHERE wallet = $1`,
			walletKey,
		).Scan(&balance, &block)
		require.NoError(t, err)
		return balance, block
	}

	// (1) Initial event-path insert at block 12345 → both fields recorded.
	err = e.upsertBalanceUpdates(ctx, []balanceUpdate{
		{addr: addr, bal: big.NewInt(100), block: 12345},
	})
	require.NoError(t, err)
	balance, block := read(t)
	assert.Equal(t, "100", balance)
	require.NotNil(t, block)
	assert.Equal(t, int64(12345), *block)

	// (2) Stale-refresh upsert with block=0 → balance updates, block is
	// preserved (NOT overwritten with 0/NULL).
	err = e.upsertBalanceUpdates(ctx, []balanceUpdate{
		{addr: addr, bal: big.NewInt(200), block: 0},
	})
	require.NoError(t, err)
	balance, block = read(t)
	assert.Equal(t, "200", balance, "stale refresh should update balance")
	require.NotNil(t, block, "stale refresh must NOT clear the existing blocknumber")
	assert.Equal(t, int64(12345), *block,
		"stale refresh must NOT overwrite a real blocknumber with 0/NULL")

	// (3) Event upsert with a LOWER block → balance updates, block does
	// NOT regress (GREATEST keeps the higher existing value).
	err = e.upsertBalanceUpdates(ctx, []balanceUpdate{
		{addr: addr, bal: big.NewInt(300), block: 100},
	})
	require.NoError(t, err)
	balance, block = read(t)
	assert.Equal(t, "300", balance)
	require.NotNil(t, block)
	assert.Equal(t, int64(12345), *block,
		"event with a lower block must not regress blocknumber from a higher previous value")

	// (4) Event upsert with a HIGHER block → block advances.
	err = e.upsertBalanceUpdates(ctx, []balanceUpdate{
		{addr: addr, bal: big.NewInt(400), block: 99999},
	})
	require.NoError(t, err)
	balance, block = read(t)
	assert.Equal(t, "400", balance)
	require.NotNil(t, block)
	assert.Equal(t, int64(99999), *block)
}

// TestUpsertBalanceUpdates_InsertWithNullBlock covers the cold-start case
// where a wallet is first observed via the stale-refresh path (e.g. one
// of the multi-wallet placeholder rows from the backfill SQL) rather
// than via a live event. block=0 must insert as NULL, not 0.
func TestUpsertBalanceUpdates_InsertWithNullBlock(t *testing.T) {
	pool := database.CreateTestDatabase(t, "test_jobs")
	defer pool.Close()

	ctx := context.Background()
	_, err := pool.Exec(ctx, migration0203SQL)
	require.NoError(t, err)

	e := &EthIndexer{pool: pool}
	addr := common.HexToAddress("0xabcdefabcdefabcdefabcdefabcdefabcdefabcd")

	err = e.upsertBalanceUpdates(ctx, []balanceUpdate{
		{addr: addr, bal: big.NewInt(500), block: 0},
	})
	require.NoError(t, err)

	var balance string
	var block *int64
	err = pool.QueryRow(ctx,
		`SELECT balance::text, blocknumber FROM eth_wallet_balances WHERE wallet = $1`,
		lowerHex(addr),
	).Scan(&balance, &block)
	require.NoError(t, err)
	assert.Equal(t, "500", balance)
	assert.Nil(t, block, "block=0 must insert as NULL, not 0")
}
