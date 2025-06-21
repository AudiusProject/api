package esindexer

import (
	"context"
	"log"
	"slices"

	"bridgerton.audius.co/config"
	"github.com/elastic/go-elasticsearch/v8"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/tidwall/sjson"
)

func mustDialPostgres() *pgxpool.Pool {
	connConfig, err := pgxpool.ParseConfig(config.Cfg.DbUrl)
	if err != nil {
		panic(err)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), connConfig)
	if err != nil {
		panic(err)
	}
	return pool
}

func mustDialElasticsearch() *elasticsearch.Client {
	esc, err := Dial(config.Cfg.EsUrl)
	if err != nil {
		log.Fatalf("Error creating the client: %s", err)
	}
	return esc
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

func reindexCollection(i *EsIndexer, collection string) {

	if err := i.createIndex(collection); err != nil {
		panic(err)
	}
	if err := i.indexAll(collection); err != nil {
		panic(err)
	}
}

func Reindex(pool *pgxpool.Pool, esc *elasticsearch.Client, drop bool, collections ...string) {

	baseIndexer := &EsIndexer{
		pool,
		esc,
		drop,
	}

	reindexAll := len(collections) == 0 || slices.Contains(collections, "all")

	if reindexAll || slices.Contains(collections, "playlists") {
		reindexCollection(baseIndexer, "playlists")
	}
	if reindexAll || slices.Contains(collections, "tracks") {
		reindexCollection(baseIndexer, "tracks")
	}
	if reindexAll || slices.Contains(collections, "users") {
		reindexCollection(baseIndexer, "users")
	}
	if reindexAll || slices.Contains(collections, "socials") {
		reindexCollection(baseIndexer, "socials")
	}

}

func ReindexLegacy(drop bool, collections ...string) {
	pool := mustDialPostgres()
	esc := mustDialElasticsearch()
	Reindex(pool, esc, drop, collections...)
}
