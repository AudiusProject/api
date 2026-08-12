package api

import (
	"context"
	"net/http"
	"sync"
	"time"

	"api.audius.co/config"
	"go.uber.org/zap"
)

// Registration on chain says a node is *entitled* to serve content; it does
// not say the box is up. Until this existed the rendezvous rotation was built
// straight from the registry, so a node that had gone away kept being handed
// out as the primary host for whatever CIDs hashed to it, and the only remedy
// was a human noticing and shipping a new config.DeadNodes entry.
//
// That gap was live on 2026-08-11: audius.zeogrid.com was refusing
// connections but still registered, and it came back as the primary host on
// roughly a third of requests for the CIDs it owned. The blobs were fine and
// replicated — the URL just pointed at a dead box, so the browser fell back
// to whatever it had cached and users reported their new profile picture or
// album art "reverting" on refresh. ~8% of sampled artists had it in their
// host set.
const (
	// How long a single node's health probe may take.
	hostProbeTimeout = 5 * time.Second

	// How many probes run at once.
	hostProbeConcurrency = 16

	// Consecutive failures before a node leaves the rotation. Paired with the
	// 1-minute nodesPoller this is a ~3 minute ejection, slow enough to ride
	// out a restart or a blip and fast enough to matter. A single success
	// resets the count and re-admits the node.
	hostEjectAfterFailures = 3

	// Never eject more than this share of the registry. If most probes fail
	// the likely cause is this API's own network, not the whole network going
	// down at once, and emptying the rotation would break every asset URL we
	// serve. Fail open and keep the registry as-is.
	hostMaxEjectedFraction = 0.5
)

// contentHostHealth tracks consecutive health-probe failures per endpoint so
// unreachable nodes can be kept out of the rendezvous rotation.
type contentHostHealth struct {
	lock     sync.Mutex
	failures map[string]int
	client   *http.Client
}

func newContentHostHealth() *contentHostHealth {
	return &contentHostHealth{
		failures: map[string]int{},
		client:   &http.Client{Timeout: hostProbeTimeout},
	}
}

// filterLive probes every node and returns those fit to serve content.
//
// Nodes are only dropped once they've failed hostEjectAfterFailures probes in
// a row, and the whole filter fails open if that would remove more than
// hostMaxEjectedFraction of the registry.
func (h *contentHostHealth) filterLive(ctx context.Context, nodes []config.Node, logger *zap.Logger) []config.Node {
	if len(nodes) == 0 {
		return nodes
	}

	results := h.probeAll(ctx, nodes)

	h.lock.Lock()
	live := make([]config.Node, 0, len(nodes))
	ejected := make([]string, 0)
	registered := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		registered[node.Endpoint] = true
		if results[node.Endpoint] {
			delete(h.failures, node.Endpoint)
			live = append(live, node)
			continue
		}
		h.failures[node.Endpoint]++
		if h.failures[node.Endpoint] >= hostEjectAfterFailures {
			ejected = append(ejected, node.Endpoint)
			continue
		}
		// Failing but not yet past the threshold — keep serving it.
		live = append(live, node)
	}
	// Forget endpoints that have left the registry entirely.
	for endpoint := range h.failures {
		if !registered[endpoint] {
			delete(h.failures, endpoint)
		}
	}
	h.lock.Unlock()

	if float64(len(ejected)) > hostMaxEjectedFraction*float64(len(nodes)) {
		logger.Warn("content host health check failing too broadly to trust; keeping all registered nodes",
			zap.Int("would_eject", len(ejected)),
			zap.Int("registered", len(nodes)),
		)
		return nodes
	}

	if len(ejected) > 0 {
		logger.Warn("excluding unreachable content hosts from rendezvous",
			zap.Strings("endpoints", ejected),
			zap.Int("registered", len(nodes)),
		)
	}
	return live
}

// probeAll returns endpoint -> reachable.
func (h *contentHostHealth) probeAll(ctx context.Context, nodes []config.Node) map[string]bool {
	results := make(map[string]bool, len(nodes))
	var (
		mu sync.Mutex
		wg sync.WaitGroup
	)
	sem := make(chan struct{}, hostProbeConcurrency)

	for _, node := range nodes {
		wg.Add(1)
		go func(endpoint string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			ok := h.probe(ctx, endpoint)
			mu.Lock()
			results[endpoint] = ok
			mu.Unlock()
		}(node.Endpoint)
	}
	wg.Wait()
	return results
}

// probe reports whether the node answers its health check. Any non-2xx or
// transport error counts as a failure.
func (h *contentHostHealth) probe(ctx context.Context, endpoint string) bool {
	ctx, cancel := context.WithTimeout(ctx, hostProbeTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"/health_check", nil)
	if err != nil {
		return false
	}
	resp, err := h.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
