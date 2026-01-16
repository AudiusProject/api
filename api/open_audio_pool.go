package api

import (
	"math/rand"
	"net/http"

	"github.com/OpenAudio/go-openaudio/pkg/sdk"
)

type OpenAudioPool struct {
	endpoints []string
	clients   []*sdk.OpenAudioSDK
}

func NewOpenAudioPool(urls []string, httpClient *http.Client) *OpenAudioPool {
	clients := make([]*sdk.OpenAudioSDK, 0, len(urls))
	for _, u := range urls {
		if u == "" {
			continue
		}
		clients = append(clients, sdk.NewOpenAudioSDKWithClient(u, httpClient))
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

func (p *OpenAudioPool) GetAll() []struct {
	Client   *sdk.OpenAudioSDK
	Endpoint string
} {
	result := make([]struct {
		Client   *sdk.OpenAudioSDK
		Endpoint string
	}, 0, len(p.clients))
	for i, client := range p.clients {
		result = append(result, struct {
			Client   *sdk.OpenAudioSDK
			Endpoint string
		}{
			Client:   client,
			Endpoint: p.endpoints[i],
		})
	}
	rand.Shuffle(len(result), func(i, j int) {
		result[i], result[j] = result[j], result[i]
	})
	return result
}
