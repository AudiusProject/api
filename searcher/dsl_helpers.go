package searcher

import (
	"encoding/json"
	"fmt"

	"github.com/tidwall/pretty"
	"github.com/tidwall/sjson"
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
							"factor": 100,
							"modifier": "log1p",
							"missing": 0
						}
					}
				]
			}
		}
	}`, innerJson, scoreField)

	// pprintJson(dsl)
	return dsl
}

func pprintJson(j string) {
	fmt.Println(string(pretty.Pretty([]byte(j))))
}

func commonIndexSettings(mapping string) string {
	mustSet := func(key string, value any) {
		var err error
		mapping, err = sjson.Set(mapping, key, value)
		if err != nil {
			panic(err)
		}
	}

	mustSet("settings.number_of_shards", 1)
	mustSet("settings.number_of_replicas", 0)
	return mapping
}
