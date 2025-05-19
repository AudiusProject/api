package searcher

import (
	"context"
	"io"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/tidwall/gjson"
)

func SearchAndPluck(esClient *elasticsearch.Client, index, dsl string, limit, offset int) ([]int32, error) {
	req := esapi.SearchRequest{
		Index:  []string{index},
		Body:   strings.NewReader(dsl),
		Source: []string{"false"},
		Size:   &limit,
		From:   &offset,
	}

	res, err := req.Do(context.Background(), esClient)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	// fmt.Println("FOUND", index, string(body))

	result := []int32{}
	for _, hit := range gjson.GetBytes(body, "hits.hits").Array() {
		id := hit.Get("_id").Int()
		result = append(result, int32(id))
	}

	return result, nil
}
