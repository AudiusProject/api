package hll

import (
	"context"
	"fmt"
	"testing"
	"time"

	"bridgerton.audius.co/database"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func setupHLL(t *testing.T) (*HLL, *pgxpool.Pool, context.Context, string) {
	t.Helper()

	ctx := context.Background()
	logger := zap.NewNop()
	pool := database.CreateTestDatabase(t, "test_hll")

	tableName := "test_hll_sketch"
	createTestTable(t, ctx, pool, tableName)

	precision := 12
	hll, err := NewHLL(logger, pool, tableName, precision)
	assert.NoError(t, err)

	return hll, pool, ctx, tableName
}

func TestHLL_BasicRecording(t *testing.T) {
	hll, _, _, _ := setupHLL(t)

	// Record some values
	testValues := []string{
		"user1",
		"user2",
		"user3",
		"user1",
		"user2",
		"user4",
	}
	for _, value := range testValues {
		hll.Record(value)
	}

	// Check estimates
	estimate := hll.GetEstimate()
	totalRequests := hll.GetTotalRequests()

	assert.Equal(t, int64(6), totalRequests)
	assert.Equal(t, uint64(4), estimate)
}

func TestHLL_AggregationWorks(t *testing.T) {
	hll, pool, ctx, tableName := setupHLL(t)

	// Record some values first
	testValues := []string{
		"user1",
		"user2",
		"user3",
		"user1",
		"user2",
		"user4",
	}
	for _, value := range testValues {
		hll.Record(value)
	}

	// Get a copy of the sketch and reset
	sketch, totalRequests := hll.GetSketchCopy()
	assert.NotNil(t, sketch)
	assert.Equal(t, totalRequests, int64(6))

	// Verify HLL was reset
	assert.Equal(t, uint64(0), hll.GetEstimate())
	assert.Equal(t, int64(0), hll.GetTotalRequests())

	// Test aggregation
	dateBucket := time.Now().Format("2006-01-02")
	tx, err := pool.Begin(ctx)
	defer tx.Rollback(ctx)
	err = hll.AggregateSketch(ctx, tx, sketch, totalRequests, dateBucket)
	assert.NoError(t, err)
	err = tx.Commit(ctx)

	// Verify data was stored
	var sketchData []byte
	var totalCount, uniqueCount int64

	query := fmt.Sprintf(`
		SELECT hll_sketch, total_count, unique_count 
		FROM %s 
		WHERE date = $1`, tableName)

	err = pool.QueryRow(ctx, query, dateBucket).Scan(&sketchData, &totalCount, &uniqueCount)
	assert.NoError(t, err)

	assert.Equal(t, totalRequests, totalCount)
	assert.Equal(t, uniqueCount, int64(4))
	assert.NotEmpty(t, sketchData)
}

func TestHLL_MergeExistingSketch(t *testing.T) {
	hll, pool, ctx, tableName := setupHLL(t)

	// Record initial values
	initialValues := []string{
		"user1",
		"user2",
		"user3",
		"user1",
		"user2",
		"user4",
	}
	for _, value := range initialValues {
		hll.Record(value)
	}

	// Get first sketch and aggregate it
	sketch1, totalRequests1 := hll.GetSketchCopy()
	require.NotNil(t, sketch1)

	dateBucket := time.Now().Format("2006-01-02")

	// Aggregate first sketch
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)
	err = hll.AggregateSketch(ctx, tx, sketch1, totalRequests1, dateBucket)
	require.NoError(t, err)
	err = tx.Commit(ctx)
	require.NoError(t, err)

	// Record more values
	moreValues := []string{
		"user5",
		"user6",
		"user1",
		"user7",
	} // user1 is duplicate
	for _, value := range moreValues {
		hll.Record(value)
	}

	// Get second sketch copy
	sketch2, totalRequests2 := hll.GetSketchCopy()
	assert.NotNil(t, sketch2)

	// Aggregate again (should merge with existing)
	tx, err = pool.Begin(ctx)
	assert.NoError(t, err)
	defer tx.Rollback(ctx)

	err = hll.AggregateSketch(ctx, tx, sketch2, totalRequests2, dateBucket)
	assert.NoError(t, err)

	err = tx.Commit(ctx)
	assert.NoError(t, err)

	// Verify merged data
	var totalCount, uniqueCount int64

	query := fmt.Sprintf(`
		SELECT total_count, unique_count 
		FROM %s 
		WHERE date = $1`, tableName)

	err = pool.QueryRow(ctx, query, dateBucket).Scan(&totalCount, &uniqueCount)
	assert.NoError(t, err)

	// Should have more total requests after merge
	assert.Equal(t, totalCount, int64(10))
	assert.Equal(t, uniqueCount, int64(7))
}

func TestHLL_GetStats(t *testing.T) {
	hll, _, _, _ := setupHLL(t)

	// Record some values for stats
	hll.Record("test1")
	hll.Record("test2")

	stats := hll.GetStats()

	assert.Contains(t, stats, "hll_unique_count")
	assert.Contains(t, stats, "hll_total_count")
	assert.Equal(t, int64(2), stats["hll_total_count"])
}

func createTestTable(t *testing.T, ctx context.Context, pool *pgxpool.Pool, tableName string) {
	createSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			date DATE PRIMARY KEY,
			hll_sketch BYTEA NOT NULL,
			total_count BIGINT NOT NULL,
			unique_count BIGINT NOT NULL,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL
		)`, tableName)

	_, err := pool.Exec(ctx, createSQL)
	require.NoError(t, err)
}
