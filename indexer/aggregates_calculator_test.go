package indexer

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestAggregatesCalculatorStartReturnsOnCancelledContext(t *testing.T) {
	a := &AggregatesCalculator{logger: zap.NewNop()}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	done := make(chan error, 1)
	go func() { done <- a.Start(ctx) }()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}
