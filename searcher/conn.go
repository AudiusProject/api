package searcher

import "github.com/elastic/go-elasticsearch/v8"

func Dial(esUrl string) (*elasticsearch.Client, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{
			esUrl,
		},
	}

	return elasticsearch.NewClient(cfg)
}
