package api

import (
	"math/rand"
	"sync"
	"time"

	"github.com/OpenAudio/go-openaudio/pkg/sdk"
	"go.uber.org/zap"
)

// OpenAudioPool holds a set of OpenAudio SDK clients and returns a random one per Get().
// Thread-safe for concurrent callers.
type OpenAudioPool struct {
	endpoints []string
	clients   []*sdk.OpenAudioSDK
	logger    *zap.Logger

	mu     sync.Mutex
	randSrc *rand.Rand
}

func NewOpenAudioPool(urls []string, logger *zap.Logger) *OpenAudioPool {
	clients := make([]*sdk.OpenAudioSDK, 0, len(urls))
	for _, u := range urls {
		if u == "" {
			continue
		}
		clients = append(clients, sdk.NewOpenAudioSDK(u))
	}
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	return &OpenAudioPool{
		endpoints: urls,
		clients:   clients,
		logger:    logger,
		randSrc:   r,
	}
}

// Get returns a random client and its endpoint.
// If no clients are configured, it returns nil, "".
func (p *OpenAudioPool) Get() (*sdk.OpenAudioSDK, string) {
	if len(p.clients) == 0 {
		return nil, ""
	}
	if len(p.clients) == 1 {
		return p.clients[0], p.endpoints[0]
	}
	p.mu.Lock()
	idx := p.randSrc.Intn(len(p.clients))
	p.mu.Unlock()
	return p.clients[idx], p.endpoints[idx]
}


