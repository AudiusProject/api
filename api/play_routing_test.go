package api

import (
	"net/url"
	"testing"

	"api.audius.co/api/dbv1"
	"github.com/stretchr/testify/require"
)

const streamPath = "/tracks/cidstream/abc?signature=sig"

func hostsOf(t *testing.T, link *dbv1.MediaLink) []string {
	t.Helper()
	u, err := url.Parse(link.Url)
	require.NoError(t, err)
	return append([]string{u.Host}, link.Mirrors...)
}

// Unconfigured, this must be exactly inert -- it ships ahead of the migration
// and sits dormant in production until someone sets the env var.
func TestPlayRoutingIsInertWhenUnconfigured(t *testing.T) {
	link := &dbv1.MediaLink{Url: "https://node-a.example" + streamPath, Mirrors: []string{"node-b.example"}}
	require.Same(t, link, withPlayRoutingHosts(link, nil))
	require.Same(t, link, withPlayRoutingHosts(link, []string{}))
	require.Nil(t, withPlayRoutingHosts(nil, []string{"x.example"}))
}

// Routing hosts go first, because tryFindWorkingUrl probes in order and the
// first host that can serve is the one that records the play.
func TestPlayRoutingHostsAreTriedFirst(t *testing.T) {
	link := &dbv1.MediaLink{Url: "https://node-a.example" + streamPath, Mirrors: []string{"node-b.example"}}
	routed := withPlayRoutingHosts(link, []string{"creatornode.audius.co", "v.monophonic.digital"})

	require.Equal(t,
		[]string{"creatornode.audius.co", "v.monophonic.digital", "node-a.example", "node-b.example"},
		hostsOf(t, routed))
}

// The original hosts stay as fallbacks. Store-all nodes hold nearly everything,
// but a freshly uploaded track may not have replicated yet, and it still has to
// be streamable -- just with the play landing on whichever node serves it.
func TestPlayRoutingKeepsOriginalHostsAsFallback(t *testing.T) {
	link := &dbv1.MediaLink{Url: "https://node-a.example" + streamPath, Mirrors: []string{"node-b.example"}}
	routed := withPlayRoutingHosts(link, []string{"creatornode.audius.co"})

	require.Contains(t, hostsOf(t, routed), "node-a.example")
	require.Contains(t, hostsOf(t, routed), "node-b.example")
}

// The path and query -- including the signature mediorum parses to attribute the
// listen -- must survive the host rewrite, or the play is recorded against the
// wrong user or not at all.
func TestPlayRoutingPreservesPathAndSignature(t *testing.T) {
	link := &dbv1.MediaLink{Url: "https://node-a.example" + streamPath}
	routed := withPlayRoutingHosts(link, []string{"creatornode.audius.co"})

	u, err := url.Parse(routed.Url)
	require.NoError(t, err)
	require.Equal(t, "creatornode.audius.co", u.Host)
	require.Equal(t, "/tracks/cidstream/abc", u.Path)
	require.Equal(t, "sig", u.Query().Get("signature"))
}

// A routing host that is already the primary or a mirror must not be probed
// twice; duplicates would waste a request and could double-count if one of them
// ever lost its skip_play_count.
func TestPlayRoutingDeduplicatesHosts(t *testing.T) {
	link := &dbv1.MediaLink{Url: "https://creatornode.audius.co" + streamPath, Mirrors: []string{"node-b.example"}}
	routed := withPlayRoutingHosts(link, []string{"creatornode.audius.co", "node-b.example"})

	require.Equal(t, []string{"creatornode.audius.co", "node-b.example"}, hostsOf(t, routed))
}

// Hosts may be configured bare or as full URLs; both must normalise to the same
// thing so a scheme in the env var does not silently create a duplicate.
func TestPlayRoutingAcceptsBareHostsAndUrls(t *testing.T) {
	link := &dbv1.MediaLink{Url: "https://node-a.example" + streamPath}
	bare := withPlayRoutingHosts(link, []string{"creatornode.audius.co"})
	full := withPlayRoutingHosts(link, []string{"https://creatornode.audius.co"})
	require.Equal(t, hostsOf(t, bare), hostsOf(t, full))
}

// An unparseable link is returned untouched rather than dropped: streaming
// should degrade to current behaviour, never fail, because of this feature.
func TestPlayRoutingLeavesUnparseableLinkAlone(t *testing.T) {
	link := &dbv1.MediaLink{Url: "://not a url"}
	require.Same(t, link, withPlayRoutingHosts(link, []string{"creatornode.audius.co"}))
}
