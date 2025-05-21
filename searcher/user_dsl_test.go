package searcher

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/pretty"
)

func TestUserDsl(t *testing.T) {
	query := "ray"

	esClient, err := Dial()
	require.NoError(t, err)

	dsl := fmt.Sprintf(`
	{
		"query": {
			"function_score": {
				"query": {
					"simple_query_string": {
						"query": %q,
						"default_operator": "AND"
					}
				},
				"boost_mode": "sum",
				"score_mode": "sum",
				"functions": [
					{
						"field_value_factor": {
							"field": "follower_count",
							"factor": 1,
							"modifier": "log1p",
							"missing": 0
						}
					}
				]
			}
		}
	}`, query+"*")

	fmt.Println(dsl)

	req := esapi.SearchRequest{
		Index: []string{"tracks"},
		Body:  strings.NewReader(dsl),
	}

	res, err := req.Do(context.Background(), esClient)
	require.NoError(t, err)

	cool := pretty.Pretty([]byte(res.String()))
	fmt.Println(string(cool))
}
