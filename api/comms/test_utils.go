package comms

import (
	"testing"

	"bridgerton.audius.co/api/dbv1"
	"bridgerton.audius.co/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// CreateTestValidator creates a validator instance for testing
func CreateTestValidator(t *testing.T, pool *pgxpool.Pool) *Validator {
	limiter, err := NewRateLimiter()
	require.NoError(t, err)

	// Create a minimal test config
	testConfig := &config.Config{
		Env:              "test",
		AntiAbuseOracles: []string{}, // Empty for tests
	}

	// Create a test logger
	logger := zap.NewNop()

	// Create DBPools for the validator
	dbPools := &dbv1.DBPools{
		Replicas: []*pgxpool.Pool{pool},
	}

	// Create validator
	return NewValidator(dbPools, limiter, testConfig, logger)
}
