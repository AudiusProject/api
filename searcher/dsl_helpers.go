package searcher

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/pretty"
)

func toAnySlice[T any](slice []T) []any {
	result := make([]any, len(slice))
	for i, v := range slice {
		result[i] = v
	}
	return result
}

func BuildFunctionScoreDSL(scoreField string, innerQuery map[string]any) string {
	innerJson, err := json.Marshal(innerQuery)
	if err != nil {
		panic(err)
	}

	dsl := fmt.Sprintf(`
	{
		"query": {
			"function_score": {
				"query": %s,
				"boost_mode": "sum",
				"score_mode": "sum",
				"functions": [
					{
						"field_value_factor": {
							"field": %q,
							"factor": 1,
							"modifier": "log1p",
							"missing": 0
						}
					}
				]
			}
		}
	}`, innerJson, scoreField)

	pprintJson(dsl)
	return dsl
}

func pprintJson(j string) {
	fmt.Println(string(pretty.Pretty([]byte(j))))
}
