// copy-pasted from mediorum placement.go

package rendezvous

import (
	"bytes"
	"crypto/sha256"
	"io"
	"math/rand"
	"net/url"
	"slices"
	"sort"
	"strings"

	"bridgerton.audius.co/config"
)

var GlobalHasher *RendezvousHasher

func init() {
	GlobalHasher = NewRendezvousHasher(config.Cfg.Nodes)
}

type HostTuple struct {
	host  string
	score []byte
}

type HostTuples []HostTuple

func (s HostTuples) Len() int      { return len(s) }
func (s HostTuples) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s HostTuples) Less(i, j int) bool {
	c := bytes.Compare(s[i].score, s[j].score)
	if c == 0 {
		return s[i].host < s[j].host
	}
	return c == -1
}

func NewRendezvousHasher(hosts []string) *RendezvousHasher {
	deadHosts := strings.Join(config.Cfg.DeadNodes, ",")
	liveHosts := make([]string, 0, len(hosts))
	for _, h := range hosts {
		// dead host
		if strings.Contains(deadHosts, h) {
			continue
		}

		// invalid url
		if _, err := url.Parse(h); err != nil {
			continue
		}

		// duplicate entry
		if slices.Contains(liveHosts, h) {
			continue
		}

		liveHosts = append(liveHosts, h)
	}
	return &RendezvousHasher{
		hosts: liveHosts,
	}
}

type RendezvousHasher struct {
	hosts []string
}

func (rh *RendezvousHasher) Rank(key string) []string {
	tuples := make(HostTuples, len(rh.hosts))
	keyBytes := []byte(key)
	hasher := sha256.New()
	for idx, host := range rh.hosts {
		hasher.Reset()
		io.WriteString(hasher, host)
		hasher.Write(keyBytes)
		tuples[idx] = HostTuple{host, hasher.Sum(nil)}
	}
	sort.Sort(tuples)
	result := make([]string, len(rh.hosts))
	for idx, tup := range tuples {
		result[idx] = tup.host
	}
	return result
}

// Get a replica set of 3 nodes with random order
func (rh *RendezvousHasher) ReplicaSet3(key string) (string, []string) {
	ranked := rh.Rank(key)
	n := min(len(ranked), 3)
	if n == 0 {
		return "", []string{}
	}

	candidates := append([]string(nil), ranked[:n]...)
	rand.Shuffle(n, func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	return candidates[0], candidates[1:]
}
