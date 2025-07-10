package hll

import (
	"context"
	"sync"

	"github.com/axiomhq/hyperloglog"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
)

// HLL handles HyperLogLog sketch operations for cardinality estimation
// It buckets unique counts into daily buckets and aggregates them in a single table
type HLL struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	writePool     *pgxpool.Pool
	sketch        *hyperloglog.Sketch
	totalRequests int64
	serverId      string
	tableName     string
	precision     uint8
}

// NewHLL creates a new HLL cardinality tracker
// Example
// hll := hll.NewHLL(logger, writePool, serverId, tableName, precision)
// where
//
// serverId is a unique identifier for the server
//
// tableName is the name of a table with the following columns:
// - date_bucket: date bucket (YYYY-MM-DD)
// - hll_sketch: hyperloglog sketch
// - total_count: total number of requests
// - unique_count: unique number of requests
// - updated_at: timestamp of the last update
func NewHLL(
	logger *zap.Logger,
	writePool *pgxpool.Pool,
	serverId string,
	tableName string,
	precision int,
) *HLL {
	// Create HLL sketch with specified precision
	sketch, err := hyperloglog.NewSketch(uint8(precision), true)
	if err != nil {
		logger.Error("Failed to create HLL sketch", zap.Error(err))
		sketch = hyperloglog.New() // fallback to default
	}

	return &HLL{
		logger:        logger.With(zap.String("component", "HLL")),
		writePool:     writePool,
		sketch:        sketch,
		totalRequests: 0,
		serverId:      serverId,
		tableName:     tableName,
		precision:     uint8(precision),
	}
}

// Record adds a value to the HLL sketch for cardinality estimation
func (h *HLL) Record(value string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Add value to HLL sketch
	h.sketch.Insert([]byte(value))
	h.totalRequests++
}

// GetEstimate returns the current estimate of unique values
func (h *HLL) GetEstimate() uint64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.sketch.Estimate()
}

// GetTotalRequests returns the total number of requests recorded
func (h *HLL) GetTotalRequests() int64 {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.totalRequests
}

// GetSketchCopy returns a copy of the current sketch and total requests, then resets the internal state
func (h *HLL) GetSketchCopy() (*hyperloglog.Sketch, int64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Create a copy of the current sketch
	sketchCopy, err := hyperloglog.NewSketch(h.precision, true)
	if err != nil {
		h.logger.Error("Failed to create sketch copy", zap.Error(err))
		return nil, 0
	}

	// Marshal and unmarshal to create a deep copy
	data, err := h.sketch.MarshalBinary()
	if err != nil {
		h.logger.Error("Failed to marshal sketch for copy", zap.Error(err))
		return nil, 0
	}

	if err := sketchCopy.UnmarshalBinary(data); err != nil {
		h.logger.Error("Failed to unmarshal sketch copy", zap.Error(err))
		return nil, 0
	}

	totalRequests := h.totalRequests

	// Reset the internal sketch and counter
	newSketch, err := hyperloglog.NewSketch(h.precision, true)
	if err != nil {
		h.logger.Error("Failed to create new sketch", zap.Error(err))
		return sketchCopy, totalRequests
	}

	h.sketch = newSketch
	h.totalRequests = 0

	return sketchCopy, totalRequests
}

// AggregateSketch directly aggregates HLL data into the specified table
func (h *HLL) AggregateSketch(ctx context.Context, tx pgx.Tx, sketch *hyperloglog.Sketch, totalRequests int64, dateBucket string) error {
	// First, try to get existing daily sketch with row-level lock
	var existingSketchData []byte
	var existingCount int64

	query := `
		SELECT hll_sketch, total_count 
		FROM ` + h.tableName + ` 
		WHERE date_bucket = $1 
		FOR UPDATE`

	err := tx.QueryRow(ctx, query, dateBucket).Scan(&existingSketchData, &existingCount)

	if err != nil && err != pgx.ErrNoRows {
		return err
	}

	if err == pgx.ErrNoRows {
		// No existing sketch - insert new one
		newSketchData, err := sketch.MarshalBinary()
		if err != nil {
			return err
		}

		estimatedUnique := int64(sketch.Estimate())

		insertQuery := `
			INSERT INTO ` + h.tableName + ` (date_bucket, hll_sketch, total_count, unique_count, updated_at)
			VALUES ($1, $2, $3, $4, NOW())`

		_, err = tx.Exec(ctx, insertQuery, dateBucket, newSketchData, totalRequests, estimatedUnique)

		return err
	} else {
		// Merge with existing sketch
		existingSketch, err := hyperloglog.NewSketch(h.precision, true)
		if err != nil {
			return err
		}

		if err := existingSketch.UnmarshalBinary(existingSketchData); err != nil {
			return err
		}

		if err := existingSketch.Merge(sketch); err != nil {
			return err
		}

		mergedData, err := existingSketch.MarshalBinary()
		if err != nil {
			return err
		}

		newTotalRequests := existingCount + totalRequests
		estimatedUnique := int64(existingSketch.Estimate())

		updateQuery := `
			UPDATE ` + h.tableName + ` 
			SET hll_sketch = $2, total_count = $3, unique_count = $4, updated_at = NOW()
			WHERE date_bucket = $1`

		_, err = tx.Exec(ctx, updateQuery, dateBucket, mergedData, newTotalRequests, estimatedUnique)

		return err
	}
}

// GetStats returns HLL statistics
func (h *HLL) GetStats() map[string]interface{} {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return map[string]interface{}{
		"hll_unique_count": h.sketch.Estimate(),
		"hll_total_count":  h.totalRequests,
		"server_id":        h.serverId,
	}
}
