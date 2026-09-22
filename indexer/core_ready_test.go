package indexer

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	corev1 "github.com/OpenAudio/go-openaudio/pkg/api/core/v1"
	corev1connect "github.com/OpenAudio/go-openaudio/pkg/api/core/v1/v1connect"
	"github.com/OpenAudio/go-openaudio/pkg/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type fakeCoreClient struct {
	corev1connect.CoreServiceClient

	mu        sync.Mutex
	calls     int
	failFirst int
}

func (f *fakeCoreClient) GetNodeInfo(context.Context, *connect.Request[corev1.GetNodeInfoRequest]) (*connect.Response[corev1.GetNodeInfoResponse], error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.calls++
	if f.failFirst < 0 || f.calls <= f.failFirst {
		return nil, connect.NewError(connect.CodeUnavailable, errors.New("core service not ready"))
	}
	return connect.NewResponse(&corev1.GetNodeInfoResponse{Chainid: "test-chain"}), nil
}

func (f *fakeCoreClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls
}

func newTestCoreIndexer(core corev1connect.CoreServiceClient) *CoreIndexer {
	return &CoreIndexer{
		logger:       zap.NewNop(),
		openAudioSDK: &sdk.OpenAudioSDK{Core: core},
	}
}

func TestAwaitCoreReadyRetriesUntilCoreIsUp(t *testing.T) {
	core := &fakeCoreClient{failFirst: 3}
	ci := newTestCoreIndexer(core)

	err := ci.awaitCoreReady(context.Background(), 5*time.Second, time.Millisecond)

	require.NoError(t, err)
	assert.Equal(t, 4, core.callCount())
}

func TestAwaitCoreReadyTimesOut(t *testing.T) {
	core := &fakeCoreClient{failFirst: -1}
	ci := newTestCoreIndexer(core)

	err := ci.awaitCoreReady(context.Background(), 50*time.Millisecond, time.Millisecond)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "core service not ready after")
	assert.NotErrorIs(t, err, context.Canceled)
}

func TestAwaitCoreReadyReturnsCanceledOnShutdown(t *testing.T) {
	core := &fakeCoreClient{failFirst: -1}
	ci := newTestCoreIndexer(core)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := ci.awaitCoreReady(ctx, time.Minute, time.Millisecond)

	assert.ErrorIs(t, err, context.Canceled)
}
