package rendezvous

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestReplicaSet3(t *testing.T) {
	hosts := []string{
		"https://host1.com",
		"https://host2.com",
		"https://host3.com",
		"https://host4.com",
		"https://host5.com",
	}

	hasher := NewRendezvousHasher(hosts)
	first, rest := hasher.ReplicaSet3("test-key")
	ranked := hasher.Rank("test-key")

	assert.Equal(t, 2, len(rest))
	assert.Contains(t, ranked, first)
	for i := 0; i < 2; i++ {
		assert.Contains(t, ranked, rest[i])
	}
}
