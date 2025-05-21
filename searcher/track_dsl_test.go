package searcher

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/aquasecurity/esquery"
	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/pretty"
)

type TrachSearchQuery struct {
	Query       string
	MinBPM      int
	MaxBPM      int
	Genre       string
	MusicalKeys []string
}

func (t *TrachSearchQuery) Map() map[string]any {
	builder := esquery.Bool()

	if t.Query != "" {
		builder.Must(esquery.Match("title", t.Query))
	}

	if t.MinBPM > 0 || t.MaxBPM > 0 {
		bpmRange := esquery.Range("bpm")
		if t.MinBPM > 0 {
			bpmRange.Gte(t.MinBPM)
		}
		if t.MaxBPM > 0 {
			bpmRange.Lte(t.MaxBPM)
		}
		builder.Filter(bpmRange)
	}

	if t.Genre != "" {
		builder.Filter(esquery.Match("genre", t.Genre))
	}

	if len(t.MusicalKeys) > 0 {
		keys := []any{}
		for _, k := range t.MusicalKeys {
			keys = append(keys, k)
		}
		builder.Filter(esquery.Terms("musical_key.keyword", keys...))
	}

	return builder.Map()
}

func (t *TrachSearchQuery) MustJSON() []byte {
	innerDsl, err := json.Marshal(t.Map())
	if err != nil {
		panic(err)
	}
	return innerDsl
}

func TestTrackDsl(t *testing.T) {
	esClient, err := Dial()
	require.NoError(t, err)

	ts := TrachSearchQuery{
		// Query:  "fever",
		MinBPM:      80,
		MaxBPM:      220,
		Genre:       "Rap",
		MusicalKeys: []string{"A minor", "B minor"},
	}

	{
		j := ts.MustJSON()
		fmt.Println(string(pretty.Pretty(j)))
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
							"field": "repost_count",
							"factor": 1,
							"modifier": "log1p",
							"missing": 0
						}
					}
				]
			}
		}
	}`, ts.MustJSON())

	fmt.Println(string(pretty.Pretty([]byte(dsl))))

	req := esapi.SearchRequest{
		Index: []string{"tracks"},
		Body:  strings.NewReader(dsl),
	}

	res, err := req.Do(context.Background(), esClient)
	require.NoError(t, err)

	cool := pretty.Pretty([]byte(res.String()))
	fmt.Println(string(cool))

}

func pprint(m map[string]any) {
	j, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(j))
}
