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
	t.Helper()
	t.Parallel()

	ctx := context.Background()
	var err error

	dbName := fmt.Sprintf("testdb_%d", rand.Int())
	{
		testMutex.Lock()
		defer testMutex.Unlock()
		if testPoolForCreatingChildDatabases == nil {
			testPoolForCreatingChildDatabases, err = pgxpool.New(ctx, "postgres://postgres:example@localhost:21300/test01")
			require.NoError(t, err)
		}
		_, err = testPoolForCreatingChildDatabases.Exec(ctx, "CREATE DATABASE "+dbName+" TEMPLATE test01")
		require.NoError(t, err)
	}

	connString := "postgres://postgres:example@localhost:21300/" + dbName
	pool, err := pgxpool.New(t.Context(), connString)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()

		testMutex.Lock()
		defer testMutex.Unlock()

		_, err := testPoolForCreatingChildDatabases.Exec(ctx, "DROP DATABASE IF EXISTS "+dbName)
		require.NoError(t, err)

	})
	return pool
}
