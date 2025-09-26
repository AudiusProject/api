package database

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"testing"

	"api.audius.co/config"
	"api.audius.co/ddl"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var testMutex = sync.Mutex{}

// Creates a new test database based on the template
// TODO: Make this require a *testing.T rather than bifurcating on whether t is nil.
func CreateTestDatabase(t *testing.T, template string) *pgxpool.Pool {
	if t != nil {
		t.Helper()
		t.Parallel()
	}

	ctx := context.Background()
	var err error

	dbName := fmt.Sprintf("testdb_%d", rand.Int())
	{
		testMutex.Lock()
		defer testMutex.Unlock()

		conn, err := pgx.Connect(ctx, "postgres://postgres:example@localhost:21300/"+template)
		if err != nil {
			panic(fmt.Errorf("failed to connect to database: %w", err))
		}
		defer conn.Close(ctx)

		_, err = conn.Exec(ctx, "CREATE DATABASE "+dbName+" TEMPLATE "+template)
		if err != nil {
			panic(fmt.Errorf("failed to create test database: %w", err))
		}
	}

	connString := "postgres://postgres:example@localhost:21300/" + dbName
	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		panic(fmt.Errorf("failed to create test database pool: %w", err))
	}

	if t != nil {
		// t.Cleanup(func() {
		// 	pool.Close()

		// 	testMutex.Lock()
		// 	defer testMutex.Unlock()

		// 	conn, err := pgx.Connect(ctx, "postgres://postgres:example@localhost:21300/"+template)
		// 	require.NoError(t, err)
		// 	defer conn.Close(ctx)

		// 	_, err = conn.Exec(ctx, "DROP DATABASE IF EXISTS "+dbName)
		// 	require.NoError(t, err)

		// })
	}
	config.Cfg.RunMigrations = true
	config.Cfg.WriteDbUrl = connString
	fmt.Println("Current working directory:", func() string { dir, _ := os.Getwd(); return dir }())
	err = ddl.RunMigrations()
	if err != nil {
		panic(err)
	}
	return pool
}
