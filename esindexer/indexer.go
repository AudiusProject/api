package esindexer

import (
	"context"
	"fmt"
	"log"
	"strconv"
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
	bulk esutil.BulkIndexer
	drop bool
}

func (indexer *EsIndexer) createIndex(collection string) error {
	cc := collectionConfigs[collection]

	if indexer.drop {
		_, err := indexer.esc.Indices.Delete([]string{cc.indexName})
		if err != nil {
			fmt.Println("drop error", cc.indexName, err)
		}
	}

	res, err := indexer.esc.Indices.Create(
		cc.indexName,
		indexer.esc.Indices.Create.WithBody(
			strings.NewReader(cc.mapping),
		),
	)
	if err != nil {
		fmt.Println("create index error", cc.indexName, err)
		return err
	}

	fmt.Println("created index", cc.indexName)
	return res.Body.Close()
}

func (indexer *EsIndexer) indexAll(collection string) error {
	cc := collectionConfigs[collection]

	err := indexer.indexSql(cc.indexName, cc.sql)
	if err != nil {
		return err
	}

	fmt.Printf("stats: %s %+v \n", collection, indexer.bulk.Stats())

	return nil
}

func (indexer *EsIndexer) indexIds(collection string, ids ...int64) error {
	cc := collectionConfigs[collection]
	stringIds := make([]string, len(ids))
	for idx, id := range ids {
		stringIds[idx] = strconv.Itoa(int(id))
	}

	sql := fmt.Sprintf("%s WHERE %s IN (%s)",
		cc.sql,
		cc.idColumn,
		strings.Join(stringIds, ","))

	return indexer.indexSql(cc.indexName, sql)
}

func (indexer *EsIndexer) indexSql(indexName, sql string) error {

	ctx := context.Background()

	rows, err := indexer.pool.Query(ctx, sql)
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

		err = indexer.bulk.Add(ctx, esutil.BulkIndexerItem{
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

	return rows.Err()
}
