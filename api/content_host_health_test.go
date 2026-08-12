package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"api.audius.co/config"
	"go.uber.org/zap"
)

func healthyServer(t *testing.T) *httptest.Server {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health_check" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(s.Close)
	return s
}

// deadEndpoint returns a URL nothing is listening on.
func deadEndpoint(t *testing.T) string {
	t.Helper()
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := s.URL
	s.Close()
	return url
}

func nodesFor(endpoints ...string) []config.Node {
	nodes := make([]config.Node, 0, len(endpoints))
	for _, e := range endpoints {
		nodes = append(nodes, config.Node{Endpoint: e, ServiceType: "content-node"})
	}
	return nodes
}

func endpointsOf(nodes []config.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.Endpoint)
	}
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// A node that answers its health check is never dropped.
func TestContentHostHealth_KeepsHealthyNodes(t *testing.T) {
	h := newContentHostHealth()
	a, b := healthyServer(t), healthyServer(t)
	nodes := nodesFor(a.URL, b.URL)

	for i := 0; i < hostEjectAfterFailures+2; i++ {
		live := h.filterLive(context.Background(), nodes, zap.NewNop())
		if len(live) != 2 {
			t.Fatalf("round %d: live = %v, want both nodes", i, endpointsOf(live))
		}
	}
}

// Regression (prod, 2026-08-11): audius.zeogrid.com was registered on chain
// but refusing connections, and kept being handed out as the primary host for
// the CIDs that hashed to it — so users saw their new profile picture or album
// art "revert" while the browser fell back to a cached copy. An unreachable
// node must leave the rotation, but only after it has failed consistently.
func TestContentHostHealth_EjectsUnreachableNodeAfterThreshold(t *testing.T) {
	h := newContentHostHealth()
	good1, good2, good3 := healthyServer(t), healthyServer(t), healthyServer(t)
	dead := deadEndpoint(t)
	nodes := nodesFor(good1.URL, good2.URL, good3.URL, dead)

	// Below the threshold the node is still served: one blip shouldn't pull a
	// node out from under live traffic.
	for i := 1; i < hostEjectAfterFailures; i++ {
		live := endpointsOf(h.filterLive(context.Background(), nodes, zap.NewNop()))
		if !contains(live, dead) {
			t.Fatalf("probe %d: dead node ejected too early, live = %v", i, live)
		}
	}

	live := endpointsOf(h.filterLive(context.Background(), nodes, zap.NewNop()))
	if contains(live, dead) {
		t.Errorf("dead node still in rotation after %d failures: %v", hostEjectAfterFailures, live)
	}
	if len(live) != 3 {
		t.Errorf("live = %v, want the three healthy nodes", live)
	}
}

// A node that comes back is re-admitted, and its failure count resets so the
// next outage gets a full threshold again.
func TestContentHostHealth_ReadmitsRecoveredNode(t *testing.T) {
	h := newContentHostHealth()
	good1, good2, good3 := healthyServer(t), healthyServer(t), healthyServer(t)

	flaky := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer flaky.Close()
	nodes := nodesFor(good1.URL, good2.URL, good3.URL, flaky.URL)

	for i := 0; i < hostEjectAfterFailures; i++ {
		h.filterLive(context.Background(), nodes, zap.NewNop())
	}
	if live := endpointsOf(h.filterLive(context.Background(), nodes, zap.NewNop())); contains(live, flaky.URL) {
		t.Fatalf("setup: expected flaky node ejected, live = %v", live)
	}

	flaky.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	live := endpointsOf(h.filterLive(context.Background(), nodes, zap.NewNop()))
	if !contains(live, flaky.URL) {
		t.Errorf("recovered node not re-admitted: %v", live)
	}

	h.lock.Lock()
	failures := h.failures[flaky.URL]
	h.lock.Unlock()
	if failures != 0 {
		t.Errorf("failure count = %d after recovery, want 0", failures)
	}
}

// If most probes fail the likely cause is this API's own network, not the
// whole network. Emptying the rotation would break every asset URL, so the
// filter fails open.
func TestContentHostHealth_FailsOpenWhenMostNodesUnreachable(t *testing.T) {
	h := newContentHostHealth()
	good := healthyServer(t)
	nodes := nodesFor(good.URL, deadEndpoint(t), deadEndpoint(t), deadEndpoint(t))

	var live []config.Node
	for i := 0; i < hostEjectAfterFailures; i++ {
		live = h.filterLive(context.Background(), nodes, zap.NewNop())
	}
	if len(live) != len(nodes) {
		t.Errorf("live = %v, want all %d nodes kept (fail open)", endpointsOf(live), len(nodes))
	}
}

// An empty registry passes through untouched rather than being probed.
func TestContentHostHealth_EmptyRegistry(t *testing.T) {
	h := newContentHostHealth()
	if live := h.filterLive(context.Background(), nil, zap.NewNop()); len(live) != 0 {
		t.Errorf("live = %v, want empty", endpointsOf(live))
	}
}

// Endpoints that leave the registry stop being tracked.
func TestContentHostHealth_ForgetsDeregisteredEndpoints(t *testing.T) {
	h := newContentHostHealth()
	good1, good2, good3 := healthyServer(t), healthyServer(t), healthyServer(t)
	dead := deadEndpoint(t)

	h.filterLive(context.Background(), nodesFor(good1.URL, good2.URL, good3.URL, dead), zap.NewNop())
	h.lock.Lock()
	tracked := h.failures[dead]
	h.lock.Unlock()
	if tracked == 0 {
		t.Fatalf("setup: expected a recorded failure for %s", dead)
	}

	h.filterLive(context.Background(), nodesFor(good1.URL, good2.URL, good3.URL), zap.NewNop())
	h.lock.Lock()
	_, stillTracked := h.failures[dead]
	h.lock.Unlock()
	if stillTracked {
		t.Errorf("failure count for deregistered %s should have been dropped", dead)
	}
}
