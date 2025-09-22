package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"api.audius.co/config"

	"go.uber.org/zap"
)

// ContentNodeMonitor pings storage enabled nodes on a timer and caches a list
// of the ones that responded.
type ContentNodeMonitor struct {
	config              config.Config
	storageEnabledNodes []config.Node // Filtered list of nodes with storage enabled
	healthyNodes        []config.Node
	mu                  sync.RWMutex
	stopChan            chan struct{}
	running             bool
	runningMu           sync.Mutex
	httpClient          *http.Client
	logger              *zap.Logger
}

func NewContentNodeMonitor(cfg config.Config, logger *zap.Logger) *ContentNodeMonitor {
	// Filter nodes to only include those with storage enabled
	var storageEnabledNodes []config.Node
	for _, node := range cfg.Nodes {
		if !node.IsStorageDisabled {
			storageEnabledNodes = append(storageEnabledNodes, node)
		}
	}

	return &ContentNodeMonitor{
		config:              cfg,
		storageEnabledNodes: storageEnabledNodes,
		healthyNodes:        storageEnabledNodes,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
		stopChan: make(chan struct{}),
		logger:   logger,
	}
}

func (m *ContentNodeMonitor) Start() error {
	m.runningMu.Lock()
	defer m.runningMu.Unlock()

	if m.running {
		return fmt.Errorf("monitor is already running")
	}

	m.running = true

	go m.monitorLoop()

	return nil
}

func (m *ContentNodeMonitor) Stop() {
	m.runningMu.Lock()
	defer m.runningMu.Unlock()

	if !m.running {
		return
	}

	close(m.stopChan)
	m.running = false
}

func (m *ContentNodeMonitor) GetContentNodes() []config.Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make([]config.Node, len(m.healthyNodes))
	copy(result, m.healthyNodes)
	return result
}

// monitorLoop runs the main monitoring loop.
func (m *ContentNodeMonitor) monitorLoop() {
	timePeriod := m.config.ContentNodeMonitorInterval
	if timePeriod == 0 {
		timePeriod = 2 * time.Minute
	}

	ticker := time.NewTicker(timePeriod)
	defer ticker.Stop()

	// Perform initial health check
	m.updateHealthyNodes()

	for {
		select {
		case <-ticker.C:
			m.updateHealthyNodes()
		case <-m.stopChan:
			return
		}
	}
}

func (m *ContentNodeMonitor) updateHealthyNodes() {
	// Use a mutex to ensure only one health check runs at a time
	m.runningMu.Lock()
	defer m.runningMu.Unlock()

	if !m.running {
		return
	}

	// Create a context with timeout for the entire health check process
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Channel to collect results
	type healthResult struct {
		node    config.Node
		healthy bool
	}

	resultChan := make(chan healthResult, len(m.storageEnabledNodes))

	// Check health in parallel
	for _, node := range m.storageEnabledNodes {
		go func(n config.Node) {
			healthy := m.checkSingleNodeHealth(ctx, n)
			resultChan <- healthResult{node: n, healthy: healthy}
		}(node)
	}

	var healthyNodes []config.Node

nodeCheckLoop:
	for i := 0; i < len(m.storageEnabledNodes); i++ {
		select {
		case result := <-resultChan:
			if result.healthy {
				healthyNodes = append(healthyNodes, result.node)
			}
		// Done indicates timeout, so we break and stop checking
		case <-ctx.Done():
			m.logger.Error("Content node health check timed out",
				zap.Error(ctx.Err()),
				zap.Int("nodes_checked", i),
				zap.Int("total_nodes", len(m.storageEnabledNodes)))
			break nodeCheckLoop
		}
	}

	m.mu.Lock()
	m.healthyNodes = healthyNodes
	m.mu.Unlock()

	m.logger.Debug("Content node health check completed",
		zap.Int("healthy_nodes", len(healthyNodes)),
		zap.Int("total_nodes", len(m.storageEnabledNodes)))
}

func (m *ContentNodeMonitor) checkSingleNodeHealth(ctx context.Context, node config.Node) bool {
	const maxRetries = 3

	for attempt := range maxRetries {
		if m.checkNodeEndpoint(ctx, node) {
			return true
		}

		// Retry w/ simple backoff
		if attempt < maxRetries-1 {
			select {
			case <-time.After(time.Duration(attempt+1) * time.Second):
			case <-ctx.Done():
				return false
			}
		}
	}

	return false
}

func (m *ContentNodeMonitor) checkNodeEndpoint(ctx context.Context, node config.Node) bool {
	healthURL := fmt.Sprintf("%s/health_check", node.Endpoint)

	req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
	if err != nil {
		return false
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}
