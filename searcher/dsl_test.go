package searcher

import (
	"context"
	"strings"
	"testing"

	"github.com/elastic/go-elasticsearch/v8/esapi"
	"github.com/stretchr/testify/require"
)

func testSearch(t *testing.T, indexName, dsl string) {
	t.Helper()

	pprintJson(dsl)

	esClient, err := Dial()
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
