package dbv1

import (
	"context"
	"math/rand"

	"bridgerton.audius.co/logging"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type DBPools struct {
	Replicas []*pgxpool.Pool
}

// NewDBPools creates a new DBPools struct from a list of database connection strings.
// It parses each connection string, configures SQL logging if not in test environment,
// and creates connection pools for each replica.
func NewDBPools(connectionStrings []string, logger *zap.Logger, env string, zapLevel zapcore.Level) (*DBPools, error) {
	var pools []*pgxpool.Pool

	for _, connStr := range connectionStrings {
		connConfig, err := pgxpool.ParseConfig(connStr)
		if err != nil {
			return nil, err
		}

		// Configure SQL logging if not in test environment
		if env != "test" {
			connConfig.ConnConfig.Tracer = &tracelog.TraceLog{
				Logger:   logging.NewSqlLogger(logger, zapLevel),
				LogLevel: tracelog.LogLevelTrace, // capture everything into sql logger
			}
		}

		pool, err := pgxpool.NewWithConfig(context.Background(), connConfig)
		if err != nil {
			return nil, err
		}

		pools = append(pools, pool)
	}

	return &DBPools{
		Replicas: pools,
	}, nil
}

// Close closes all connection pools in the DBPools struct.
func (pools *DBPools) Close() {
	for _, pool := range pools.Replicas {
		if pool != nil {
			pool.Close()
		}
	}
}

// Query executes a query and returns the rows.
func (pools *DBPools) Query(ctx context.Context, sql string, arguments ...interface{}) (pgx.Rows, error) {
	pool := ChooseReplica(pools.Replicas)
	if pool == nil {
		return nil, pgx.ErrNoRows
	}
	return pool.Query(ctx, sql, arguments...)
}

// QueryRow executes a query and returns a single row.
func (pools *DBPools) QueryRow(ctx context.Context, sql string, arguments ...interface{}) pgx.Row {
	pool := ChooseReplica(pools.Replicas)
	if pool == nil {
		// Return a row that will error when scanned
		return &errorRow{err: pgx.ErrNoRows}
	}
	return pool.QueryRow(ctx, sql, arguments...)
}

// Exec executes a query that doesn't return rows.
func (pools *DBPools) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	pool := ChooseReplica(pools.Replicas)
	if pool == nil {
		return pgconn.CommandTag{}, pgx.ErrNoRows
	}
	return pool.Exec(ctx, sql, arguments...)
}

func ChooseReplica(replicas []*pgxpool.Pool) *pgxpool.Pool {

	// Simple random selection
	n := len(replicas)
	if n == 0 {
		return nil
	}
	return replicas[rand.Intn(n)]
}

// errorRow is a pgx.Row implementation that always returns an error
type errorRow struct {
	err error
}

func (r *errorRow) Scan(dest ...interface{}) error {
	return r.err
}
