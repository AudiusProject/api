package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"testing"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
)

type mappy map[string]any

func toJsonReader(thing any) io.Reader {
	b, err := json.Marshal(thing)
	if err != nil {
		panic(err)
	}
	return bytes.NewReader(b)
}

func TestOpenSearchIndexAndGet(t *testing.T) {

	indexName := "test2"

	client, err := opensearch.NewClient(opensearch.Config{
		Addresses: []string{"http://localhost:21320"},
	})
	if err != nil {
		t.Fatalf("Error creating OpenSearch client: %s", err)
	}

	// delete index
	{

		opensearchapi.IndicesDeleteRequest{
			Index: []string{indexName},
		}.Do(context.Background(), client)
		// assert.NoError(t, err)
		// fmt.Println(ok)
	}

	// create index
	{
		settings := mappy{
			"settings": mappy{
				"index": mappy{
					"number_of_shards":   1,
					"number_of_replicas": 0,
				},
			},
		}

		req := opensearchapi.IndicesCreateRequest{
			Index: indexName,
			Body:  toJsonReader(settings),
		}

		res, err := req.Do(context.Background(), client)
		if err != nil {
			panic(err)
		}
		fmt.Println(res)
	}

	// Index a document
	docID := "1"
	doc := map[string]string{"title": "Test Document"}
	docJSON, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("Error marshaling document: %s", err)
	}

	req := opensearchapi.IndexRequest{
		Index:      indexName,
		DocumentID: docID,
		Body:       bytes.NewReader(docJSON),
		Refresh:    "true",
	}
	res, err := req.Do(context.Background(), client)
	if err != nil {
		t.Fatalf("Error indexing document: %s", err)
	}
	defer res.Body.Close()

	if res.IsError() {
		t.Fatalf("Error response from OpenSearch: %s", res.String())
	}

	// Get the document by ID
	getReq := opensearchapi.GetRequest{
		Index:      indexName,
		DocumentID: docID,
	}
	getRes, err := getReq.Do(context.Background(), client)
	if err != nil {
		t.Fatalf("Error getting document: %s", err)
	}
	defer getRes.Body.Close()

	if getRes.IsError() {
		t.Fatalf("Error response from OpenSearch: %s", getRes.String())
	}

	var retrievedDoc map[string]interface{}
	if err := json.NewDecoder(getRes.Body).Decode(&retrievedDoc); err != nil {
		t.Fatalf("Error decoding response body: %s", err)
	}

	source, ok := retrievedDoc["_source"].(map[string]interface{})
	if !ok {
		t.Fatalf("Error extracting _source from response")
	}

	if source["title"] != doc["title"] {
		t.Errorf("Expected title %s, got %s", doc["title"], source["title"])
	}

	fmt.Println(source)
}
