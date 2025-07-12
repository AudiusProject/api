package database

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/test-go/testify/require"
)

var testMutex = sync.Mutex{}
var testPoolForCreatingChildDatabases *pgxpool.Pool

func NewTestDatabase(t *testing.T) *pgxpool.Pool {
	ctx := context.Background()
	var err error

	if testPoolForCreatingChildDatabases == nil {
		testPoolForCreatingChildDatabases, err = pgxpool.New(ctx, "postgres://postgres:example@localhost:21300/test01")
		require.NoError(t, err)
	}

	dbName := fmt.Sprintf("testdb_%d", rand.Int())
	testMutex.Lock()
	_, err = testPoolForCreatingChildDatabases.Exec(ctx, "CREATE DATABASE "+dbName+" TEMPLATE test01")
	testMutex.Unlock()
	require.NoError(t, err)

	connString := "postgres://postgres:example@localhost:21300/" + dbName
	pool, err := pgxpool.New(t.Context(), connString)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
		testMutex.Lock()
		_, err := testPoolForCreatingChildDatabases.Exec(ctx, "DROP DATABASE IF EXISTS test")
		testMutex.Unlock()
		require.NoError(t, err)
	})
	return pool
}
