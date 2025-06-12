package searcher

import (
	"context"
	"strings"
	"testing"

	"bridgerton.audius.co/utils"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/stretchr/testify/require"
)

func testSearch(t *testing.T, indexName, dsl string) {
	t.Helper()

	pprintJson(dsl)

	esClient, err := Dial("http://localhost:21400")
	require.NoError(t, err)

	req := esapi.SearchRequest{
		Index: []string{indexName},
		Body:  strings.NewReader(dsl),
	}

	res, err := req.Do(context.Background(), esClient)
	require.NoError(t, err)
	body := res.String()

	if res.IsError() {
		require.FailNow(t, body)
	}

	pprintJson(body)
	// todo: assert not empty and stuff...
}

func TestCommonIndexSettings(t *testing.T) {
	{
		mapping := `{
			"mappings": {
				"properties": {
					"name": {
						"type": "text"
					},
					"handle": {
						"type": "text"
					},
					"location": {
						"type": "text"
					},
					"follower_count": {
						"type": "integer"
					},
					"track_count": {
						"type": "integer"
					}
				}
			}
		}`

		m2 := commonIndexSettings(mapping)

		utils.JsonAssert(t, []byte(m2), map[string]any{
			"mappings.properties.name.type": "text",
			"settings.number_of_shards":     1,
			"settings.number_of_replicas":   0,
		})
	}

	// works with empty mapping
	{
		m2 := commonIndexSettings(``)
		utils.JsonAssert(t, []byte(m2), map[string]any{
			"settings.number_of_shards":   1,
			"settings.number_of_replicas": 0,
		})
	}
}
