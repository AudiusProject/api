package dbv1

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNewDBPools(t *testing.T) {
	// Create a test logger
	logger := zap.NewNop()

	// Test with empty connection strings
	pools, err := NewDBPools([]string{}, logger, "test", zapcore.InfoLevel)
	if err != nil {
		t.Fatalf("Expected no error for empty connection strings, got: %v", err)
	}
	if len(pools.Replicas) != 0 {
		t.Errorf("Expected 0 replicas, got %d", len(pools.Replicas))
	}

	// Test with invalid connection string
	_, err = NewDBPools([]string{"invalid://connection"}, logger, "test", zapcore.InfoLevel)
	if err == nil {
		t.Error("Expected error for invalid connection string, got nil")
	}
}

func TestChooseReplica(t *testing.T) {
	// Test with empty replicas
	result := ChooseReplica([]*pgxpool.Pool{})
	if result != nil {
		t.Errorf("Expected nil for empty replicas, got %v", result)
	}

	// Test with nil replicas
	result = ChooseReplica(nil)
	if result != nil {
		t.Errorf("Expected nil for nil replicas, got %v", result)
	}
}

func TestDBPoolsProxyMethods(t *testing.T) {
	// Test proxy methods with empty replicas
	pools := &DBPools{Replicas: []*pgxpool.Pool{}}
	ctx := context.Background()

	// Test Query
	_, err := pools.Query(ctx, "SELECT 1")
	if err == nil {
		t.Error("Expected error for Query with no replicas, got nil")
	}

	// Test QueryRow
	row := pools.QueryRow(ctx, "SELECT 1")
	var result int
	err = row.Scan(&result)
	if err == nil {
		t.Error("Expected error for QueryRow with no replicas, got nil")
	}

	// Test Exec
	_, err = pools.Exec(ctx, "SELECT 1")
	if err == nil {
		t.Error("Expected error for Exec with no replicas, got nil")
	}
}
