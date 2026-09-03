package api

import (
	"net/url"
	"strings"

	"api.audius.co/api/dbv1"
)

// withPlayRoutingHosts returns a copy of link whose candidate hosts are the
// configured routing hosts first, then the link's own url and mirrors.
//
// A play is recorded by whichever node serves the audio -- logTrackListen runs
// at the top of mediorum's serveBlob, before it 307s to storage -- and plays
// never travel through the relay. So the host in the URL the API hands back is
// what decides which chain the play lands on.
//
// During the genesis migration the fleet is split across two chains for days.
// An already-migrated node writes its plays to the new chain while the indexer
// is still reading the old one, and those plays are indexed by nobody. Naming
// hosts that stay on the old chain keeps every play on the chain the indexer is
// actually reading. Cleared at the cutover, after which plays follow the node
// serving them again. See cmd/genesis-writer/ROLLOUT.md, Runbook steps 5 and 13.
//
// The original url and mirrors are kept as fallbacks rather than replaced. The
// routing hosts are store-all nodes and hold essentially everything, but
// replication of a fresh upload is not instant, and a track they do not have
// yet must still be streamable. tryFindWorkingUrl probes in order, so a routing
// host that cannot serve costs one request and falls through.
func withPlayRoutingHosts(link *dbv1.MediaLink, hosts []string) *dbv1.MediaLink {
	if link == nil || len(hosts) == 0 {
		return link
	}

	primary, err := url.Parse(link.Url)
	if err != nil {
		return link
	}

	routed := &dbv1.MediaLink{}
	seen := make(map[string]bool, len(hosts)+len(link.Mirrors)+1)

	// Host-only comparison: mirrors are recorded as hosts, and the routing hosts
	// are configured as hosts, so normalising to that avoids listing the same
	// node twice under different spellings.
	add := func(raw string) {
		h := hostOf(raw)
		if h == "" || seen[h] {
			return
		}
		seen[h] = true
		if routed.Url == "" {
			u := *primary
			u.Host = h
			routed.Url = u.String()
			return
		}
		routed.Mirrors = append(routed.Mirrors, h)
	}

	for _, h := range hosts {
		add(h)
	}
	add(link.Url)
	for _, m := range link.Mirrors {
		add(m)
	}

	if routed.Url == "" {
		return link
	}
	return routed
}

// hostOf accepts either a bare host or a full URL and returns the host.
func hostOf(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if !strings.Contains(s, "//") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return ""
	}
	return u.Host
}
