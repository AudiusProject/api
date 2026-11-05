package api

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"

	"api.audius.co/api/dbv1"
)

// recordingHost is a stand-in for a content node: it records the query string of
// every request it receives so a test can assert what the probe sent.
type recordingHost struct {
	mu      sync.Mutex
	queries []url.Values
	status  int
}

func (h *recordingHost) handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h.mu.Lock()
		h.queries = append(h.queries, r.URL.Query())
		h.mu.Unlock()
		w.WriteHeader(h.status)
	}
}

// The probe must send skip_play_count so a two-byte range request is not counted
// as a listen -- but the URL handed back to the caller must not carry it. A node
// serving /tracks/cidstream/:cid returns early from logTrackListen when it sees
// that flag, so a leaked probe artifact silently suppresses the play.
func TestReturnedUrlHasNoProbeArtifacts(t *testing.T) {
	host := &recordingHost{status: http.StatusPartialContent}
	srv := httptest.NewServer(host.handler())
	defer srv.Close()

	got := tryFindWorkingUrl(&dbv1.MediaLink{Url: srv.URL + "/tracks/cidstream/abc?signature=sig"})
	if got == nil {
		t.Fatal("expected a url")
	}

	if _, leaked := got.Query()["skip_play_count"]; leaked {
		t.Errorf("returned url carries skip_play_count=%q; the serving node will not record the play",
			got.Query().Get("skip_play_count"))
	}
	if got.Query().Get("signature") != "sig" {
		t.Errorf("signature lost from returned url: %q", got.String())
	}

	host.mu.Lock()
	defer host.mu.Unlock()
	if len(host.queries) != 1 {
		t.Fatalf("expected 1 probe, got %d", len(host.queries))
	}
	if host.queries[0].Get("skip_play_count") != "true" {
		t.Error("the probe itself must set skip_play_count, or probing inflates play counts")
	}
}

// The no-working-host fallback returns the main URL, which is also urls[0] --
// so it must not have been mutated by its own probe attempt.
func TestFallbackUrlHasNoProbeArtifacts(t *testing.T) {
	host := &recordingHost{status: http.StatusInternalServerError}
	srv := httptest.NewServer(host.handler())
	defer srv.Close()

	got := tryFindWorkingUrl(&dbv1.MediaLink{Url: srv.URL + "/tracks/cidstream/abc?signature=sig"})
	if got == nil {
		t.Fatal("expected the main url as fallback")
	}
	if _, leaked := got.Query()["skip_play_count"]; leaked {
		t.Error("fallback url carries skip_play_count from its own probe")
	}
}

// Mirrors are probed the same way and must come back equally clean.
func TestMirrorUrlHasNoProbeArtifacts(t *testing.T) {
	dead := httptest.NewServer((&recordingHost{status: http.StatusInternalServerError}).handler())
	defer dead.Close()
	live := httptest.NewServer((&recordingHost{status: http.StatusOK}).handler())
	defer live.Close()

	liveURL, _ := url.Parse(live.URL)
	got := tryFindWorkingUrl(&dbv1.MediaLink{
		Url:     dead.URL + "/tracks/cidstream/abc?signature=sig",
		Mirrors: []string{live.URL},
	})
	if got == nil {
		t.Fatal("expected the mirror")
	}
	if got.Host != liveURL.Host {
		t.Fatalf("expected mirror host %s, got %s", liveURL.Host, got.Host)
	}
	if _, leaked := got.Query()["skip_play_count"]; leaked {
		t.Error("mirror url carries skip_play_count from its probe")
	}
}
