package database

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/test-go/testify/require"
)

var testMutex = sync.Mutex{}

func NewTestDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	t.Parallel()

	ctx := context.Background()
	var err error

	dbName := fmt.Sprintf("testdb_%d", rand.Int())
	{
		testMutex.Lock()
		defer testMutex.Unlock()

		conn, err := pgx.Connect(ctx, "postgres://postgres:example@localhost:21300/test01")
		require.NoError(t, err)
		defer conn.Close(ctx)

		_, err = conn.Exec(ctx, "CREATE DATABASE "+dbName+" TEMPLATE test01")
		require.NoError(t, err)
	}

	connString := "postgres://postgres:example@localhost:21300/" + dbName
	pool, err := pgxpool.New(t.Context(), connString)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()

		testMutex.Lock()
		defer testMutex.Unlock()

		conn, err := pgx.Connect(ctx, "postgres://postgres:example@localhost:21300/test01")
		require.NoError(t, err)
		defer conn.Close(ctx)

		_, err = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+dbName)
		require.NoError(t, err)

	})
	return pool
}
