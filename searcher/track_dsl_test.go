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

func TestTrackDsl(t *testing.T) {
	esClient, err := Dial()
	require.NoError(t, err)

	query := "fever"

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
							"field": "repost_count",
							"factor": 1,
							"modifier": "log1p",
							"missing": 0
						}
					}
				]
			}
		}
	}`, query+"*")

	req := esapi.SearchRequest{
		Index: []string{"tracks"},
		Body:  strings.NewReader(dsl),
	}

	res, err := req.Do(context.Background(), esClient)
	require.NoError(t, err)

	cool := pretty.Pretty([]byte(res.String()))
	fmt.Println(string(cool))

}
