package searchv1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/tidwall/gjson"
	"github.com/tidwall/pretty"
)

func toAnySlice[T any](slice []T) []any {
	result := make([]any, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}

func sortWithField(innerQuery map[string]any, sortField, direction string) string {
	innerJson, err := json.Marshal(innerQuery)
	if err != nil {
		panic(err)
	}

	dsl := fmt.Sprintf(`
	{
		"query": %s,
		"sort": [
			{"%s": {"order": "%s"}}
		]
	}`, innerJson, sortField, direction)

	return dsl
}

// countScoreFunction scales a score by count+1. A plain field_value_factor
// multiplies by the raw count, which zeroes out anything with a count of 0
// and buries it below every other match, exact or not.
func countScoreFunction(field string) string {
	source := fmt.Sprintf("(doc['%s'].size() == 0 ? 0 : doc['%s'].value) + 1", field, field)
	return fmt.Sprintf(`"script_score": {"script": {"source": %q}}`, source)
}

func BuildFunctionScoreDSL(scoreField string, weight float64, innerQuery map[string]any) string {
	innerJson, err := json.Marshal(innerQuery)
	if err != nil {
		panic(err)
	}

	dsl := fmt.Sprintf(`
	{
		"query": {
			"function_score": {
				"query": %s,
				"functions": [
					{
						%s,
						"weight": %g
					}
				],
				"boost_mode": "multiply",
				"score_mode": "multiply"
			}
		}
	}`, innerJson, countScoreFunction(scoreField), weight)

	return dsl
}

func SearchAndPluck(esClient *elasticsearch.Client, index, dsl string, limit, offset int) ([]int32, error) {

	// set to true to debug scoring (locally)
	// don't leave in in prod tho
	explain := false

	req := esapi.SearchRequest{
		Index:   []string{index},
		Body:    strings.NewReader(dsl),
		Source:  []string{"false"},
		Size:    &limit,
		From:    &offset,
		Explain: &explain,
	}

	res, err := req.Do(context.Background(), esClient)
	if err != nil {
		return nil, err
	}
	defer func() { _ = res.Body.Close() }()

	if res.IsError() {
		return nil, fmt.Errorf("search %s failed: %s", index, res.String())
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if explain {
		pprintJson(string(body))
	}

	result := []int32{}
	for _, hit := range gjson.GetBytes(body, "hits.hits").Array() {
		id := hit.Get("_id").Int()
		result = append(result, int32(id))
	}

	return result, nil
}

func pprintJson(j string) {
	fmt.Println(string(pretty.Pretty([]byte(j))))
}
