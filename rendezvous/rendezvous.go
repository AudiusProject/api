package rendezvous

import (
	"fmt"
	"math/rand"
	"sync"

	mediorum "github.com/AudiusProject/audiusd/pkg/mediorum/server"
)

var GlobalHasher *RendezvousHasher

func init() {
	// updated once eth nodes are retrieved
	GlobalHasher = NewRendezvousHasher([]string{})
}

type RendezvousHasher struct {
	hasher *mediorum.RendezvousHasher
	mu     sync.Mutex
}

func NewRendezvousHasher(hosts []string) *RendezvousHasher {
	return &RendezvousHasher{
		hasher: mediorum.NewRendezvousHasher(hosts),
	}
}

func (rh *RendezvousHasher) Rank(key string) []string {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	return rh.hasher.Rank(key)
}

func (rh *RendezvousHasher) Update(hosts []string) {
	rh.mu.Lock()
	defer rh.mu.Unlock()
	rh.hasher = mediorum.NewRendezvousHasher(hosts)
}

// Get a replica set of 3 nodes with random order
func (rh *RendezvousHasher) ReplicaSet3(key string) (string, []string) {
	fmt.Println("ReplicaSet3", key)
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
