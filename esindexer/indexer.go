package esindexer

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type collectionConfig struct {
	indexName string
	idColumn  string
	mapping   string
	sql       string
}

var collectionConfigs = map[string]collectionConfig{
	"users":     userConfig,
	"tracks":    tracksConfig,
	"playlists": playlistsConfig,
	"socials":   socialsConfig,
}

type EsIndexer struct {
	pool *pgxpool.Pool
	esc  *elasticsearch.Client
	// bulk esutil.BulkIndexer
	drop bool
}

func (base *EsIndexer) createIndex(indexName string) error {
	if base.drop {
		_, err := base.esc.Indices.Delete([]string{indexName})
		if err != nil {
			fmt.Println("drop error", indexName, err)
		}
	}

	cc := collectionConfigs[indexName]

	res, err := base.esc.Indices.Create(
		indexName,
		base.esc.Indices.Create.WithBody(
			strings.NewReader(cc.mapping),
		),
	)
	if err != nil {
		fmt.Println("create index error", indexName, err)
		return err
	}

	fmt.Println("created index", indexName)
	return res.Body.Close()
}

func (base *EsIndexer) indexAll(indexName string) error {
	cc := collectionConfigs[indexName]

	ctx := context.Background()
	bulk, err := esutil.NewBulkIndexer(esutil.BulkIndexerConfig{
		Client:     base.esc,
		NumWorkers: 2,
		Refresh:    "true",
	})
	if err != nil {
		log.Fatalf("Error creating the indexer: %s", err)
	}

	rows, err := base.pool.Query(ctx, cc.sql)
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
			Index:      indexName,
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

	fmt.Printf("stats: %s %+v \n", indexName, bulk.Stats())

	return nil
}
