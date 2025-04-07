package indexer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/AudiusProject/audiusd/pkg/core/gen/core_proto"
	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/opensearch-project/opensearch-go/v2/opensearchapi"
	"github.com/stretchr/testify/assert"
)

func TestIndexToOpensearch(t *testing.T) {
	// t.Skip()

	indexName := "meng"

	client, err := opensearch.NewClient(opensearch.Config{
		Addresses: []string{"http://localhost:21320"},
	})
	if err != nil {
		t.Fatalf("Error creating OpenSearch client: %s", err)
	}

	opensearchapi.IndicesDeleteRequest{
		Index: []string{indexName},
	}.Do(context.Background(), client)

	file, err := os.Open("testdata/take1.pb")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	dr := NewDumpReader(file)

	for dr.Next() {
		signedTx := &dr.signedTx
		em := signedTx.GetManageEntity()
		if em == nil {
			continue
		}

		var m GenericMetadata
		json.Unmarshal([]byte(em.Metadata), &m)

		emfix := EntityManagerFixMetadata{
			em,
			m.Data,
		}

		docJSON, _ := json.Marshal(emfix)
		fmt.Println("YO", string(docJSON))

		req := opensearchapi.IndexRequest{
			Index: indexName,
			// DocumentID: docID,
			Body: bytes.NewReader(docJSON),
			// Refresh: "true",
		}
		res, err := req.Do(context.Background(), client)
		if err != nil {
			t.Fatalf("Error indexing document: %s", err)
		}
		defer res.Body.Close()

		assert.NoError(t, err)
	}
}

type EntityManagerFixMetadata struct {
	*core_proto.ManageEntityLegacy

	Metadata map[string]any `json:"metadata"`
}
