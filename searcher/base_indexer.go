package searcher

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tidwall/gjson"
)

type BaseIndexer struct {
	pool *pgxpool.Pool
	esc  *elasticsearch.Client
	drop bool
}

func (base *BaseIndexer) createIndex(indexName string, mapping string) error {
	if base.drop {
		_, err := base.esc.Indices.Delete([]string{indexName})
		if err != nil {
			fmt.Println("drop error", indexName, err)
		}
	}

	res, err := base.esc.Indices.Create(
		indexName,
		base.esc.Indices.Create.WithBody(
			strings.NewReader(mapping),
		),
	)
	if err != nil {
		fmt.Println("create index error", indexName, err)
		return err
	}

	fmt.Println("created index", indexName)
	return res.Body.Close()
}

func (base *BaseIndexer) bulkIndexQuery(index, sql string) error {

	// Get the max updated_at from the Elasticsearch index
	var maxBlocknumber int64

	if false {

		res, err := base.esc.Search(
			base.esc.Search.WithIndex(index),
			base.esc.Search.WithBody(strings.NewReader(`{
				"size": 0,
				"aggs": {
					"max_blocknumber": {
						"max": {
							"field": "blocknumber"
						}
					}
				}
			}`)),
			base.esc.Search.WithTrackTotalHits(false),
		)
		if err != nil {
			fmt.Println("error querying max_blocknumber:", err)
			return err
		}
		defer res.Body.Close()

		body, _ := io.ReadAll(res.Body)
		maxBlocknumber = gjson.GetBytes(body, "aggregations.max_blocknumber.value").Int()

		pprintJson(string(body))
		fmt.Println("________ maxBlocknumber", maxBlocknumber)

	}

	ctx := context.Background()
	bulk, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Index:      index,
		Client:     base.esc,
		NumWorkers: 2,
		Refresh:    "true",
	})
	if err != nil {
		log.Fatalf("Error creating the indexer: %s", err)
	}

	rows, err := base.pool.Query(ctx, sql, maxBlocknumber)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var doc string
		if err := rows.Scan(&id, &doc); err != nil {
			fmt.Println("row scan error:", err)
			continue
		}

		err = bulk.Add(ctx, esutil.BulkIndexerItem{
			Action:     "index",
			DocumentID: fmt.Sprintf("%d", id),
			Body:       strings.NewReader(doc),
			OnSuccess: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {
				// fmt.Println("index", index, id)
			},
			OnFailure: func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem, err error) {
				if err != nil {
					log.Printf("ERROR: %s", err)
				} else {
					log.Printf("ERROR: %s: %s", res.Error.Type, res.Error.Reason)
				}
			},
		})

		if err != nil {
			fmt.Println("es index error:", err)
			continue
		}
	}

	if err := rows.Err(); err != nil {
		fmt.Println("rows error:", err)
	}

	if err := bulk.Close(context.Background()); err != nil {
		log.Fatalf("Unexpected error: %s", err)
	}

	fmt.Printf("stats: %s %+v \n", index, bulk.Stats())

	return nil
}
