package esindexer

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"strconv"
	"strings"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esutil"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/sync/errgroup"
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

// create index from mapping
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

func (indexer *EsIndexer) reindexAll() error {
	g, _ := errgroup.WithContext(context.Background())
	for name := range collectionConfigs {
		name := name
		g.Go(func() error {
			return indexer.reindexCollection(name)
		})
	}
	return g.Wait()
}

// creates index mapping + indexes all documents
func (indexer *EsIndexer) reindexCollection(collection string) error {
	if err := indexer.createIndex(collection); err != nil {
		return err
	}
	return indexer.indexAll(collection)
}

// index all documents
func (indexer *EsIndexer) indexAll(collection string) error {
	cc := collectionConfigs[collection]

	err := indexer.indexSql(cc.indexName, cc.sql)
	if err != nil {
		return err
	}

	slog.Info("index all stats", "collection", collection, "stats", indexer.bulk.Stats())

	return nil
}

// index a list of IDs
func (indexer *EsIndexer) indexIds(collection string, ids ...int64) error {
	if len(ids) == 0 {
		return nil
	}

	cc := collectionConfigs[collection]
	stringIds := make([]string, len(ids))
	for idx, id := range ids {
		stringIds[idx] = strconv.Itoa(int(id))
	}

	sql := fmt.Sprintf("%s AND %s IN (%s)",
		cc.sql,
		cc.idColumn,
		strings.Join(stringIds, ","))

	slog.Info("index", "collection", collection, "ids", ids)

	return indexer.indexSql(cc.indexName, sql)
}

// runs a query + indexes documents
// assumes query returns (id, json_doc) tuples
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
			OnSuccess:  func(ctx context.Context, item esutil.BulkIndexerItem, res esutil.BulkIndexerResponseItem) {},
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

// this uses painless to 'patch' a `socials` doc to add / remove an entity ID from a list of ids.
// without forcing postgres to re-do the socials query from scratch.
// If missing (i.e. for a new user) it will fall back to re-doing from scratch.
// This might be a "too clever by half" type of thing... in which case, the listener could just use `indexIds`
// like the other collections
func (indexer *EsIndexer) scriptedUpdateSocial(userId int64, fieldName string, entityId int64, isDelete bool) {
	painlessAddEntity := `
	def user = ctx._source;
	if (!user[params.fieldName].contains(params.entityId)) {
		user[params.fieldName].add(params.entityId);
	} else {
		ctx.op = 'noop'
	}
	`

	painlessRemoveEntity := `
	def user = ctx._source;
	def idx = user[params.fieldName].indexOf(params.entityId);
	if (idx > -1) {
		user[params.fieldName].remove(idx);
	} else {
		ctx.op = 'noop'
	}
	`

	script := painlessAddEntity
	if isDelete {
		script = painlessRemoveEntity
	}

	resp, err := indexer.esc.Update(
		"socials",
		fmt.Sprintf("%d", userId),
		strings.NewReader(fmt.Sprintf(`{
			"script": {
				"source": %q,
				"lang": "painless",
				"params": {
					"fieldName": %q,
					"entityId": %d
				}
			}
		}`, script, fieldName, entityId)),
	)
	defer resp.Body.Close()

	if err != nil {
		log.Printf("scripted update error: %v", err)
	} else if resp.StatusCode != 200 {
		if err := indexer.indexIds("socials", userId); err != nil {
			slog.Error("socials indexIds failed", "user", userId, "field", fieldName, "id", entityId, "err", err)
		} else {
			slog.Info("socials indexIds", "user", userId, "field", fieldName, "id", entityId)
		}
	} else {
		slog.Info("social update", "user", userId, "field", fieldName, "id", entityId)
	}

}
