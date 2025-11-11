package api

import (
	"math/rand"

	"github.com/OpenAudio/go-openaudio/pkg/sdk"
)

type OpenAudioPool struct {
	endpoints []string
	clients   []*sdk.OpenAudioSDK
}

func NewOpenAudioPool(urls []string) *OpenAudioPool {
	clients := make([]*sdk.OpenAudioSDK, 0, len(urls))
	for _, u := range urls {
		if u == "" {
			continue
		}
		clients = append(clients, sdk.NewOpenAudioSDK(u))
	}
	return &OpenAudioPool{
		endpoints: urls,
		clients:   clients,
	}
}

func (p *OpenAudioPool) Get() (*sdk.OpenAudioSDK, string) {
	if len(p.clients) == 0 {
		return nil, ""
	}
	i := rand.Intn(len(p.clients))
	return p.clients[i], p.endpoints[i]
}
