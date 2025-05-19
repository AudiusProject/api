package searcher

import "github.com/elastic/go-elasticsearch/v8"

func Dial() (*elasticsearch.Client, error) {
	cfg := elasticsearch.Config{
		Addresses: []string{
			"http://35.238.44.255:21302",
		},
	}

	return elasticsearch.NewClient(cfg)
}
